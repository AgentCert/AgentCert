package utils

import (
	"errors"
	"strings"
	"testing"

	"github.com/litmuschaos/chaos-operator/api/litmuschaos/v1alpha1"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/graph/model"
)

func TestGenerateAccessKey(t *testing.T) {
	for _, n := range []int{0, 1, 16, 32} {
		key, err := GenerateAccessKey(n)
		if err != nil {
			t.Fatalf("GenerateAccessKey(%d) unexpected error: %v", n, err)
		}
		// base64 URL encoding produces a deterministically-sized output for n bytes.
		if n == 0 && key != "" {
			t.Errorf("GenerateAccessKey(0) = %q, want empty", key)
		}
		if n > 0 && key == "" {
			t.Errorf("GenerateAccessKey(%d) returned empty string", n)
		}
	}
}

func TestRandomString(t *testing.T) {
	tests := []struct {
		name   string
		n      int
		expLen int
	}{
		{"zero", 0, 0},
		{"negative", -5, 0},
		{"small", 8, 8},
		{"large", 40, 40},
	}
	const valid = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-"
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := RandomString(tc.n)
			if len(got) != tc.expLen {
				t.Fatalf("RandomString(%d) len = %d, want %d", tc.n, len(got), tc.expLen)
			}
			for _, r := range got {
				if !strings.ContainsRune(valid, r) {
					t.Errorf("RandomString produced invalid rune %q", r)
				}
			}
		})
	}
}

func TestAddRootIndent(t *testing.T) {
	in := []byte("a\nb\nc")
	got := AddRootIndent(in, 2)
	want := "a\n  b\n  c"
	if string(got) != want {
		t.Errorf("AddRootIndent() = %q, want %q", string(got), want)
	}

	// No newlines: unchanged.
	if string(AddRootIndent([]byte("flat"), 4)) != "flat" {
		t.Errorf("AddRootIndent on flat string should be unchanged")
	}
}

func TestContainsString(t *testing.T) {
	tests := []struct {
		name string
		s    []string
		str  string
		want bool
	}{
		{"present", []string{"a", "b", "c"}, "b", true},
		{"absent", []string{"a", "b"}, "z", false},
		{"empty slice", nil, "a", false},
		{"empty target present", []string{""}, "", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := ContainsString(tc.s, tc.str); got != tc.want {
				t.Errorf("ContainsString(%v, %q) = %v, want %v", tc.s, tc.str, got, tc.want)
			}
		})
	}
}

func TestMatchInfrastructureType(t *testing.T) {
	k8s := model.InfrastructureTypeKubernetes
	other := model.InfrastructureType("Linux")

	if !MatchInfrastructureType([]*model.InfrastructureType{&k8s}, k8s) {
		t.Errorf("expected match for present infra type")
	}
	if MatchInfrastructureType([]*model.InfrastructureType{&other}, k8s) {
		t.Errorf("expected no match for absent infra type")
	}
	// nil entries are skipped without panic.
	if MatchInfrastructureType([]*model.InfrastructureType{nil}, k8s) {
		t.Errorf("nil entry should not match")
	}
	if MatchInfrastructureType(nil, k8s) {
		t.Errorf("empty slice should not match")
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		in   float64
		want float64
	}{
		{1.239, 1.23},
		{1.2, 1.2},
		{0, 0},
		{99.999, 99.99},
	}
	for _, tc := range tests {
		if got := Truncate(tc.in); got != tc.want {
			t.Errorf("Truncate(%v) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestSplit(t *testing.T) {
	tests := []struct {
		name           string
		str            string
		before, after  string
		want           string
	}{
		{"basic", "key=value;", "key=", ";", "value"},
		{"no after found", "prefix:rest", "prefix:", ";", "rest"},
		{"middle", "a[content]b", "[", "]", "content"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := Split(tc.str, tc.before, tc.after); got != tc.want {
				t.Errorf("Split(%q,%q,%q) = %q, want %q", tc.str, tc.before, tc.after, got, tc.want)
			}
		})
	}
}

func TestGenerateUUID(t *testing.T) {
	a := GenerateUUID()
	b := GenerateUUID()
	if a == "" || b == "" {
		t.Fatal("GenerateUUID returned empty string")
	}
	if a == b {
		t.Errorf("GenerateUUID should produce unique values, got duplicate %q", a)
	}
}

type sampleStruct struct {
	Name  string
	Value string
	Ptr   *string
}

func TestCheckEmptyFields(t *testing.T) {
	ptr := "x"
	t.Run("all filled", func(t *testing.T) {
		if err := CheckEmptyFields(&sampleStruct{Name: "a", Value: "b", Ptr: &ptr}); err != nil {
			t.Errorf("expected no error, got %v", err)
		}
	})
	t.Run("empty string field", func(t *testing.T) {
		err := CheckEmptyFields(&sampleStruct{Name: "", Value: "b"})
		if err == nil || !strings.Contains(err.Error(), "Name") {
			t.Errorf("expected Name empty error, got %v", err)
		}
	})
	t.Run("nil ptr ignored", func(t *testing.T) {
		// Ptr is a *string so even when nil it must not trigger the empty check.
		if err := CheckEmptyFields(&sampleStruct{Name: "a", Value: "b", Ptr: nil}); err != nil {
			t.Errorf("nil pointer field should be ignored, got %v", err)
		}
	})
}

func TestStringPtrToString(t *testing.T) {
	v := "hello"
	if got := StringPtrToString(&v); got != "hello" {
		t.Errorf("StringPtrToString(&v) = %q, want hello", got)
	}
	if got := StringPtrToString(nil); got != "" {
		t.Errorf("StringPtrToString(nil) = %q, want empty", got)
	}
}

func TestParseGRPCError(t *testing.T) {
	t.Run("with prefix", func(t *testing.T) {
		in := errors.New(GRPCErrorPrefix + " something bad")
		got := ParseGRPCError(in)
		if got.Error() != " something bad" {
			t.Errorf("ParseGRPCError stripped result = %q", got.Error())
		}
	})
	t.Run("without prefix", func(t *testing.T) {
		in := errors.New("plain error")
		if got := ParseGRPCError(in); got.Error() != "plain error" {
			t.Errorf("ParseGRPCError = %q, want unchanged", got.Error())
		}
	})
}

func TestValidateUnits(t *testing.T) {
	tests := []struct {
		name  string
		value string
		unit  string
		want  string
	}{
		{"empty value", "", "s", ""},
		{"already has unit", "10s", "s", "10s"},
		{"bare number gets unit appended", "10", "s", "10s"},
		{"ms unit recognized", "200ms", "ms", "200ms"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := validateUnits(tc.value, tc.unit); got != tc.want {
				t.Errorf("validateUnits(%q,%q) = %q, want %q", tc.value, tc.unit, got, tc.want)
			}
		})
	}
}

func TestDecode(t *testing.T) {
	t.Run("valid base64", func(t *testing.T) {
		data, err := decode("aGVsbG8=") // "hello"
		if err != nil {
			t.Fatalf("decode error: %v", err)
		}
		if string(data) != "hello" {
			t.Errorf("decode = %q, want hello", string(data))
		}
	})
	t.Run("invalid base64", func(t *testing.T) {
		if _, err := decode("!!!notbase64!!!"); err == nil {
			t.Error("expected error for invalid base64")
		}
	})
}

func TestTransformProbe(t *testing.T) {
	t.Run("empty list", func(t *testing.T) {
		if got := TransformProbe(nil); got != nil {
			t.Errorf("TransformProbe(nil) = %v, want nil", got)
		}
	})

	t.Run("http probe appends ms timeout and bare-number units get s", func(t *testing.T) {
		in := []v1alpha1.ProbeAttributes{
			{
				Name: "p1",
				Type: string(model.ProbeTypeHTTPProbe),
				Mode: "Continuous",
				RunProperties: v1alpha1.RunProperty{
					ProbeTimeout: "5",
					Interval:     "2",
					Retry:        3,
				},
			},
		}
		out := TransformProbe(in)
		if len(out) != 1 {
			t.Fatalf("expected 1 probe, got %d", len(out))
		}
		if out[0].RunProperties.ProbeTimeout != "5ms" {
			t.Errorf("http probe timeout = %q, want 5ms", out[0].RunProperties.ProbeTimeout)
		}
		if out[0].RunProperties.Interval != "2s" {
			t.Errorf("interval = %q, want 2s", out[0].RunProperties.Interval)
		}
		if out[0].RunProperties.Retry != 3 {
			t.Errorf("retry not preserved")
		}
	})

	t.Run("initialDelaySeconds overrides initial delay", func(t *testing.T) {
		in := []v1alpha1.ProbeAttributes{
			{
				Name: "p2",
				Type: string(model.ProbeTypeCmdProbe),
				RunProperties: v1alpha1.RunProperty{
					InitialDelaySeconds: 7,
				},
			},
		}
		out := TransformProbe(in)
		if out[0].RunProperties.InitialDelay != "7s" {
			t.Errorf("InitialDelay = %q, want 7s", out[0].RunProperties.InitialDelay)
		}
	})
}
