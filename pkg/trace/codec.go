package trace

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"unicode/utf8"
)

// MarshalCanonical validates and serializes a current-version record with a
// stable field order and deterministic metadata-key ordering.
func MarshalCanonical(record Record) ([]byte, error) {
	return marshalCanonical(record, DefaultLimits())
}

func marshalCanonical(record Record, limits Limits) ([]byte, error) {
	limits = normalizeLimits(limits)
	if err := validateRecord(record, limits, false); err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(record)
	if err != nil {
		return nil, fmt.Errorf("%w: marshal: %v", ErrInvalidRecord, err)
	}
	if len(encoded) > limits.MaxRecordBytes {
		return nil, fmt.Errorf("%w: got %d bytes, maximum is %d", ErrRecordTooLarge, len(encoded), limits.MaxRecordBytes)
	}
	return encoded, nil
}

// Encoder writes one canonical JSON record per line. Encoder is not safe for
// concurrent use. A write failure is terminal because the final line may be
// partial; subsequent Encode calls return ErrEncoderFailed.
type Encoder struct {
	writer      io.Writer
	limits      Limits
	records     uint64
	bytes       int64
	lastArrival int64
	failed      error
}

// NewEncoder returns a bounded streaming encoder.
func NewEncoder(writer io.Writer, limits Limits) *Encoder {
	return &Encoder{writer: writer, limits: normalizeLimits(limits)}
}

// Encode validates and writes one record plus a newline.
func (e *Encoder) Encode(record Record) error {
	if e.failed != nil {
		return fmt.Errorf("%w: %v", ErrEncoderFailed, e.failed)
	}
	if e.writer == nil {
		e.failed = errors.New("writer is nil")
		return fmt.Errorf("%w: %v", ErrEncoderFailed, e.failed)
	}
	if e.records >= e.limits.MaxRecords {
		return fmt.Errorf("%w: maximum is %d", ErrRecordLimit, e.limits.MaxRecords)
	}
	if record.Sequence != e.records+1 {
		return fmt.Errorf("%w: sequence must be contiguous and start at 1; got %d, want %d", ErrInvalidRecord, record.Sequence, e.records+1)
	}
	if e.records > 0 && record.ArrivalOffsetNS < e.lastArrival {
		return fmt.Errorf("%w: arrival offsets must be non-decreasing", ErrInvalidRecord)
	}
	encoded, err := marshalCanonical(record, e.limits)
	if err != nil {
		return err
	}
	recordBytes := int64(len(encoded) + 1)
	if recordBytes > e.limits.MaxTraceBytes-e.bytes {
		return fmt.Errorf("%w: maximum is %d bytes", ErrTraceTooLarge, e.limits.MaxTraceBytes)
	}
	line := append(encoded, '\n')
	if err := writeAll(e.writer, line); err != nil {
		e.failed = err
		return err
	}
	e.records++
	e.bytes += recordBytes
	e.lastArrival = record.ArrivalOffsetNS
	return nil
}

func writeAll(writer io.Writer, data []byte) error {
	for len(data) > 0 {
		written, err := writer.Write(data)
		if err != nil {
			return err
		}
		if written <= 0 || written > len(data) {
			return io.ErrShortWrite
		}
		data = data[written:]
	}
	return nil
}

// Decoder reads bounded JSONL records. The first malformed or resource-limit
// error is terminal and is returned again on subsequent calls.
type Decoder struct {
	reader      *bufio.Reader
	limits      Limits
	records     uint64
	bytesRead   int64
	lastArrival int64
	failed      error
}

// NewDecoder returns a bounded streaming decoder.
func NewDecoder(reader io.Reader, limits Limits) *Decoder {
	limits = normalizeLimits(limits)
	if reader == nil {
		return &Decoder{
			limits: limits,
			failed: &DecodeError{Record: 1, Err: fmt.Errorf("%w: reader is nil", ErrInvalidRecord)},
		}
	}
	bufferSize := min(limits.MaxRecordBytes+1, 64<<10)
	bufferSize = max(bufferSize, 256)
	return &Decoder{
		reader: bufio.NewReaderSize(reader, bufferSize),
		limits: limits,
	}
}

// Decode returns the next record or io.EOF. Errors include a one-based record
// number and zero-based byte offset through DecodeError.
func (d *Decoder) Decode() (Record, error) {
	if d.failed != nil {
		return Record{}, d.failed
	}
	if d.reader == nil {
		d.failed = &DecodeError{Record: 1, Err: fmt.Errorf("%w: reader is nil", ErrInvalidRecord)}
		return Record{}, d.failed
	}
	if d.records >= d.limits.MaxRecords {
		_, err := d.reader.Peek(1)
		if errors.Is(err, io.EOF) {
			return Record{}, io.EOF
		}
		if err != nil {
			return Record{}, d.fail(d.bytesRead, err)
		}
		return Record{}, d.fail(d.bytesRead, ErrRecordLimit)
	}

	offset := d.bytesRead
	line, err := d.readLine()
	if errors.Is(err, io.EOF) && len(line) == 0 {
		return Record{}, io.EOF
	}
	if err != nil {
		return Record{}, d.fail(offset, err)
	}
	if len(line) == 0 {
		return Record{}, d.fail(offset, fmt.Errorf("%w: empty lines are not allowed", ErrInvalidRecord))
	}
	if !utf8.Valid(line) {
		return Record{}, d.fail(offset, fmt.Errorf("%w: record is not valid UTF-8", ErrInvalidRecord))
	}
	if err := validateJSONShape(line, d.limits.MaxNestingDepth); err != nil {
		return Record{}, d.fail(offset, err)
	}

	var version struct {
		SchemaVersion string `json:"schema_version"`
	}
	if err := json.Unmarshal(line, &version); err != nil {
		return Record{}, d.fail(offset, fmt.Errorf("%w: JSON: %v", ErrInvalidRecord, err))
	}
	var record Record
	if version.SchemaVersion == CurrentSchemaVersion {
		decoder := json.NewDecoder(bytes.NewReader(line))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&record); err != nil {
			return Record{}, d.fail(offset, fmt.Errorf("%w: JSON: %v", ErrInvalidRecord, err))
		}
	} else if err := json.Unmarshal(line, &record); err != nil {
		return Record{}, d.fail(offset, fmt.Errorf("%w: JSON: %v", ErrInvalidRecord, err))
	}
	if err := validateRecord(record, d.limits, true); err != nil {
		return Record{}, d.fail(offset, err)
	}
	if record.Sequence != d.records+1 {
		return Record{}, d.fail(offset, fmt.Errorf("%w: sequence must be contiguous and start at 1; got %d, want %d", ErrInvalidRecord, record.Sequence, d.records+1))
	}
	if d.records > 0 && record.ArrivalOffsetNS < d.lastArrival {
		return Record{}, d.fail(offset, fmt.Errorf("%w: arrival offsets must be non-decreasing", ErrInvalidRecord))
	}
	d.records++
	d.lastArrival = record.ArrivalOffsetNS
	return record, nil
}

func (d *Decoder) fail(offset int64, err error) error {
	d.failed = &DecodeError{
		Record:     d.records + 1,
		ByteOffset: offset,
		Err:        err,
	}
	return d.failed
}

func (d *Decoder) readLine() ([]byte, error) {
	var line []byte
	tooLarge := false
	for {
		fragment, err := d.reader.ReadSlice('\n')
		d.bytesRead += int64(len(fragment))
		if d.bytesRead > d.limits.MaxTraceBytes {
			return nil, ErrTraceTooLarge
		}

		content := fragment
		if err == nil && len(content) > 0 {
			content = content[:len(content)-1]
		}
		if !tooLarge {
			if len(line)+len(content) > d.limits.MaxRecordBytes {
				tooLarge = true
			} else {
				line = append(line, content...)
			}
		}

		switch {
		case err == nil:
			if tooLarge {
				return nil, ErrRecordTooLarge
			}
			if len(line) > 0 && line[len(line)-1] == '\r' {
				line = line[:len(line)-1]
			}
			return line, nil
		case errors.Is(err, bufio.ErrBufferFull):
			continue
		case errors.Is(err, io.EOF):
			if tooLarge {
				return nil, ErrRecordTooLarge
			}
			if len(line) == 0 && len(fragment) == 0 {
				return nil, io.EOF
			}
			return line, nil
		default:
			return nil, err
		}
	}
}

func validateJSONShape(data []byte, maxDepth int) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := walkJSON(decoder, 0, maxDepth); err != nil {
		return err
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		if err == nil {
			return fmt.Errorf("%w: multiple JSON values", ErrInvalidRecord)
		}
		return fmt.Errorf("%w: JSON: %v", ErrInvalidRecord, err)
	}
	return nil
}

func walkJSON(decoder *json.Decoder, depth, maxDepth int) error {
	if depth > maxDepth {
		return fmt.Errorf("%w: JSON nesting exceeds %d", ErrInvalidRecord, maxDepth)
	}
	token, err := decoder.Token()
	if err != nil {
		return fmt.Errorf("%w: JSON: %v", ErrInvalidRecord, err)
	}
	delimiter, ok := token.(json.Delim)
	if !ok {
		return nil
	}

	switch delimiter {
	case '{':
		seen := make(map[string]struct{})
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return fmt.Errorf("%w: JSON object key: %v", ErrInvalidRecord, err)
			}
			key, ok := keyToken.(string)
			if !ok {
				return fmt.Errorf("%w: JSON object key is not a string", ErrInvalidRecord)
			}
			if _, exists := seen[key]; exists {
				return fmt.Errorf("%w: %q", ErrDuplicateField, key)
			}
			seen[key] = struct{}{}
			if sensitiveFieldName(key) {
				return fmt.Errorf("%w: %q", ErrSensitiveField, key)
			}
			if err := walkJSON(decoder, depth+1, maxDepth); err != nil {
				return err
			}
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim('}') {
			return fmt.Errorf("%w: malformed JSON object", ErrInvalidRecord)
		}
	case '[':
		for decoder.More() {
			if err := walkJSON(decoder, depth+1, maxDepth); err != nil {
				return err
			}
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim(']') {
			return fmt.Errorf("%w: malformed JSON array", ErrInvalidRecord)
		}
	default:
		return fmt.Errorf("%w: unexpected JSON delimiter %q", ErrInvalidRecord, delimiter)
	}
	return nil
}
