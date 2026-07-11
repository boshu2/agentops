package config

import (
	"context"
	"strings"
	"testing"
)

type fakeCommandGateway struct{ saved *Config }

func (gateway *fakeCommandGateway) Resolve(string, bool) *ResolvedConfig { return &ResolvedConfig{} }
func (gateway *fakeCommandGateway) Files() (ConfigFiles, error)          { return ConfigFiles{}, nil }
func (gateway *fakeCommandGateway) Environment([]string) map[string]string {
	return map[string]string{}
}
func (gateway *fakeCommandGateway) Load() (*Config, error) { return Default(), nil }
func (gateway *fakeCommandGateway) Save(value *Config) error {
	gateway.saved = value
	return nil
}

func TestCommandServiceWriteModelsValidatesAndBuildsPatch(t *testing.T) {
	gateway := &fakeCommandGateway{}
	service := NewCommandService(gateway)
	result, err := service.WriteModels(context.Background(), ModelsWriteRequest{DefaultTier: "budget", Skill: "council=quality"})
	if err != nil {
		t.Fatal(err)
	}
	if result.DefaultTier != "budget" || result.SkillOverrides["council"] != "quality" {
		t.Fatalf("result = %+v", result)
	}
	if gateway.saved == nil || gateway.saved.Models.DefaultTier != "budget" || gateway.saved.Models.SkillOverrides["council"] != "quality" {
		t.Fatalf("saved patch = %+v", gateway.saved)
	}
}

func TestCommandServiceWriteModelsRejectsInvalidInputBeforeSave(t *testing.T) {
	for _, request := range []ModelsWriteRequest{{DefaultTier: "inherit"}, {DefaultTier: "premium"}, {Skill: "broken"}, {Skill: "council=premium"}} {
		gateway := &fakeCommandGateway{}
		_, err := NewCommandService(gateway).WriteModels(context.Background(), request)
		if err == nil || !strings.Contains(err.Error(), "invalid") {
			t.Fatalf("request %+v error = %v", request, err)
		}
		if gateway.saved != nil {
			t.Fatalf("invalid request %+v reached save", request)
		}
	}
}
