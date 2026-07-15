package adapter

import (
	"bytes"
	"fmt"

	"github.com/gaurav-gs7/InferLab/pkg/evidence"
)

// CheckConformance verifies capability integrity, exact fixture ownership,
// deterministic normalization, lossless mapping, and classification safety.
func CheckConformance(implementation Adapter, fixtures []Input) error {
	if implementation == nil {
		return fmt.Errorf("%w: adapter is nil", ErrInvalidCapabilities)
	}
	capabilities := implementation.Capabilities()
	if err := ValidateCapabilities(capabilities); err != nil {
		return err
	}
	if len(fixtures) == 0 {
		return fmt.Errorf("%w: at least one fixture is required", ErrInvalidInput)
	}
	for i, fixture := range fixtures {
		if fixture.Producer != capabilities.Producer {
			return fmt.Errorf("fixture[%d]: %w", i, ErrUnsupportedProducer)
		}
		first, err := implementation.Normalize(fixture)
		if err != nil {
			return fmt.Errorf("fixture[%d] first normalization: %w", i, err)
		}
		second, err := implementation.Normalize(fixture)
		if err != nil {
			return fmt.Errorf("fixture[%d] second normalization: %w", i, err)
		}
		firstJSON, err := CanonicalJSON(first)
		if err != nil {
			return fmt.Errorf("fixture[%d] canonical report: %w", i, err)
		}
		secondJSON, err := CanonicalJSON(second)
		if err != nil {
			return fmt.Errorf("fixture[%d] second canonical report: %w", i, err)
		}
		if !bytes.Equal(firstJSON, secondJSON) {
			return fmt.Errorf("fixture[%d]: normalization is not deterministic", i)
		}
		if first.Envelope.Classification != fixture.Classification {
			return fmt.Errorf("fixture[%d]: %w: input %s became %s", i, ErrClassification, fixture.Classification, first.Envelope.Classification)
		}
		if fixture.Classification == evidence.ClassPredicted && first.Envelope.Runtime.Origin != evidence.OriginDeclared {
			return fmt.Errorf("fixture[%d]: %w: predicted evidence did not retain declared runtime origin", i, ErrClassification)
		}
		if first.Envelope.Source.Tool != capabilities.Producer.Tool || first.Adapter != capabilities.Adapter {
			return fmt.Errorf("fixture[%d]: normalized provenance differs from capabilities", i)
		}
	}
	return nil
}
