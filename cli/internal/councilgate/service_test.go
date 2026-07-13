package councilgate

import (
	"context"
	"io"
	"testing"
)

type mapReader map[string]string

func (reader mapReader) Read(_ context.Context, path string, _ io.Reader) (string, error) {
	return reader[path], nil
}

func testVerdict(judge, contextID, family, token string) string {
	return "author: worker\njudge: " + judge + "\njudge_program: codex\njudge_model_family: " + family + "\ncontext_id: " + contextID + "\nVERDICT: " + token + "\nCOMMANDS RUN:\n  go test ./...\n"
}

func TestServiceAggregatesIndependentContexts(t *testing.T) {
	reader := mapReader{"a": testVerdict("a", "ctx-a", "gpt", "PASS"), "b": testVerdict("b", "ctx-b", "gpt", "PASS")}
	result := NewService(reader, Policy{}).Evaluate(context.Background(), Request{Paths: []string{"a", "b"}})
	if result.Outcome != OutcomePass || result.Pass != 2 || result.Contexts != 2 || result.Families != 1 {
		t.Fatalf("result = %+v", result)
	}
}

func TestServiceFailsClosedForMixedAndDuplicateContext(t *testing.T) {
	reader := mapReader{
		"pass": testVerdict("a", "ctx-a", "gpt", "PASS"),
		"fail": testVerdict("b", "ctx-b", "gpt", "FAIL"),
		"dupe": testVerdict("b", "CTX-A", "gpt", "PASS"),
	}
	service := NewService(reader, Policy{})
	if result := service.Evaluate(context.Background(), Request{Paths: []string{"pass", "fail"}}); result.Outcome != OutcomeDisagreement {
		t.Fatalf("mixed result = %+v", result)
	}
	if result := service.Evaluate(context.Background(), Request{Paths: []string{"pass", "dupe"}}); result.Outcome != OutcomeDuplicateContext {
		t.Fatalf("duplicate result = %+v", result)
	}
}

func TestServiceOptionalCrossFamilyPolicy(t *testing.T) {
	reader := mapReader{"a": testVerdict("a", "ctx-a", "gpt", "PASS"), "b": testVerdict("b", "ctx-b", "gpt", "PASS")}
	result := NewService(reader, Policy{RequireCrossFamily: true}).Evaluate(context.Background(), Request{Paths: []string{"a", "b"}})
	if result.Outcome != OutcomeCrossFamily {
		t.Fatalf("result = %+v", result)
	}
}
