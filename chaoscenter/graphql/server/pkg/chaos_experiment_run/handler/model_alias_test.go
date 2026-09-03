package handler

import (
	"testing"

	"github.com/tidwall/sjson"
)

func manifestWithModelAlias(t *testing.T, alias string) string {
	t.Helper()
	base := `{"metadata":{"annotations":{"litmuschaos.io/multiRunEnabled":"true"}}}`
	if alias == "" {
		return base
	}
	out, err := sjson.Set(base, modelAliasManifestAnnotation, alias)
	if err != nil {
		t.Fatalf("sjson.Set: %v", err)
	}
	return out
}

func TestResolveEffectiveModelAlias(t *testing.T) {
	tests := []struct {
		name         string
		override     string
		manifest     string
		wantEffalias string
		wantPersist  bool
	}{
		{
			name:         "explicit override, nothing stored yet -> use it and persist",
			override:     "qwen2.5-32b-instruct",
			manifest:     manifestWithModelAlias(t, ""),
			wantEffalias: "qwen2.5-32b-instruct",
			wantPersist:  true,
		},
		{
			name:         "no override, alias stored -> reuse stored, no persist (the [Multi-Run] path)",
			override:     "",
			manifest:     manifestWithModelAlias(t, "qwen2.5-32b-instruct"),
			wantEffalias: "qwen2.5-32b-instruct",
			wantPersist:  false,
		},
		{
			name:         "override equal to stored -> no redundant persist",
			override:     "qwen2.5-32b-instruct",
			manifest:     manifestWithModelAlias(t, "qwen2.5-32b-instruct"),
			wantEffalias: "qwen2.5-32b-instruct",
			wantPersist:  false,
		},
		{
			name:         "override changes the stored alias -> switch and persist",
			override:     "gpt-4o",
			manifest:     manifestWithModelAlias(t, "qwen2.5-32b-instruct"),
			wantEffalias: "gpt-4o",
			wantPersist:  true,
		},
		{
			name:         "no override, nothing stored -> empty (env default) and no persist",
			override:     "",
			manifest:     manifestWithModelAlias(t, ""),
			wantEffalias: "",
			wantPersist:  false,
		},
		{
			name:         "whitespace-only override is treated as no override",
			override:     "   ",
			manifest:     manifestWithModelAlias(t, "qwen2.5-32b-instruct"),
			wantEffalias: "qwen2.5-32b-instruct",
			wantPersist:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotAlias, gotPersist := resolveEffectiveModelAlias(tt.override, tt.manifest)
			if gotAlias != tt.wantEffalias {
				t.Errorf("effective alias = %q, want %q", gotAlias, tt.wantEffalias)
			}
			if gotPersist != tt.wantPersist {
				t.Errorf("persist = %v, want %v", gotPersist, tt.wantPersist)
			}
		})
	}
}
