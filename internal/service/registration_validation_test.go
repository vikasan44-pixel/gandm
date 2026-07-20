package service

import (
	"errors"
	"testing"

	"gandm/internal/models"
)

func TestValidateRegisterInputLegalForm(t *testing.T) {
	valid := RegisterInput{
		Email:       "person@example.com",
		Phone:       "+77000000000",
		CompanyName: "Алия",
		LegalForm:   models.LegalFormIndividual,
		Password:    "password1",
	}
	if err := validateRegisterInput(valid); err != nil {
		t.Fatalf("valid individual rejected: %v", err)
	}
	valid.LegalForm = "unknown"
	if err := validateRegisterInput(valid); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("invalid legal form error = %v, want ErrInvalidInput", err)
	}
}

func TestLegacyParticipantTypeUsesSelectedServices(t *testing.T) {
	tests := []struct {
		name string
		keys []string
		want models.ParticipantType
	}{
		{name: "no provider tools", want: models.ParticipantClient},
		{name: "warehouse", keys: []string{ToolManageWarehouse}, want: models.ParticipantWarehouse},
		{name: "carrier", keys: []string{ToolManageFleet}, want: models.ParticipantCarrier},
		{name: "customs", keys: []string{ToolManageCustomsDocs}, want: models.ParticipantCustomsRep},
		{name: "warehouse has stable priority", keys: []string{ToolManageCustomsDocs, ToolManageWarehouse}, want: models.ParticipantWarehouse},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tools := make([]models.Tool, len(tc.keys))
			for i, key := range tc.keys {
				tools[i].Key = key
			}
			if got := legacyParticipantType(tools); got != tc.want {
				t.Fatalf("participant type = %q, want %q", got, tc.want)
			}
		})
	}
}
