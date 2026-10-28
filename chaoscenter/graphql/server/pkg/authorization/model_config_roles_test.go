package authorization

import (
	"reflect"
	"testing"
)

func TestLLMModelRoles(t *testing.T) {
	everyRole := []string{MemberRoleOwnerString, MemberRoleExecutorString, MemberRoleViewerString}
	ownerOnly := []string{MemberRoleOwnerString}
	want := map[RoleQuery][]string{
		ListModelConfigs:      everyRole,
		ListAgentModelOptions: everyRole,
		// Writes and tests spend or replace a key, and reach LiteLLM with its
		// master key.
		CreateModelConfig:    ownerOnly,
		DeleteModelConfig:    ownerOnly,
		UpdateModelConfigKey: ownerOnly,
		TestModelConfig:      ownerOnly,
	}
	for query, roles := range want {
		if got := MutationRbacRules[query]; !reflect.DeepEqual(got, roles) {
			t.Errorf("MutationRbacRules[%s] = %v, want %v", query, got, roles)
		}
	}
}
