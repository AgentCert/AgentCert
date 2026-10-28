package chartconfig

import "testing"

func TestDefaultAgentModelAliasPrecedence(t *testing.T) {
	cases := []struct {
		name, flash, modelAlias, azure, want string
	}{
		{"FLASH_AGENT_MODEL first", "flash", "alias", "azure", "flash"},
		{"then MODEL_ALIAS", " ", "alias", "azure", "alias"},
		{"then AZURE_OPENAI_DEPLOYMENT", "", "", " azure ", "azure"},
		{"none", "", "", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("FLASH_AGENT_MODEL", tc.flash)
			t.Setenv("MODEL_ALIAS", tc.modelAlias)
			t.Setenv("AZURE_OPENAI_DEPLOYMENT", tc.azure)
			if got := DefaultAgentModelAlias(); got != tc.want {
				t.Fatalf("DefaultAgentModelAlias() = %q, want %q", got, tc.want)
			}
		})
	}
}
