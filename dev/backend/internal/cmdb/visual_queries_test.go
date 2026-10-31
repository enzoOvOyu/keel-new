package cmdb

import (
	"context"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

func maxVisualSQLParam(sql string) int {
	max := 0
	for _, match := range regexp.MustCompile(`\$([0-9]+)`).FindAllStringSubmatch(sql, -1) {
		n, _ := strconv.Atoi(match[1])
		if n > max {
			max = n
		}
	}
	return max
}
func TestVisualQuerySQLBindings(t *testing.T) {
	scope := Scope{Projects: []string{"p"}, UserID: "u", PermissionVersion: 3}
	q := url.Values{}
	tests := []struct {
		name, sql string
		args      int
	}{{"groups", visualGroupSQL(), len(visualArgs(scope, q, "")[:5])}, {"seed", visualSeedSQL(fabricPredicate("v")+" AND "+entitySearch("v"), 100), len(visualArgs(scope, q, ""))}, {"context", visualContextSQL(), len(append(scope.Args(), []string{"id"}, false))}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if maxVisualSQLParam(test.sql) != test.args {
				t.Fatalf("query requires %d parameters, got %d", maxVisualSQLParam(test.sql), test.args)
			}
			sql := regexp.MustCompile(`'(?:[^']|'')*'`).ReplaceAllString(test.sql, "")
			depth := 0
			for _, c := range sql {
				if c == '(' {
					depth++
				}
				if c == ')' {
					depth--
					if depth < 0 {
						t.Fatal("unbalanced SQL parentheses")
					}
				}
			}
			if depth != 0 {
				t.Fatal("unbalanced SQL parentheses")
			}
		})
	}
	expr, e := matrixExpressions("rack_leaf")
	if e != nil || !strings.Contains(expr[0], "type_a='server'") || !strings.Contains(expr[1], "type_b='server'") {
		t.Fatal("rack-to-leaf axes must exclude switch uplinks")
	}
	if !strings.Contains(visualCTE(), "HAVING count(DISTINCT w.id)=1") {
		t.Fatal("ambiguous port owner must not select arbitrary first device")
	}
}
func TestVisualQueryCursorScope(t *testing.T) {
	scope := Scope{Projects: []string{"b", "a"}, UserID: "user", PermissionVersion: 7}
	q := url.Values{"view": {"physical"}, "limit": {"100"}}
	key := visualQueryKey(scope, "subgraph", q)
	token := visualToken{Kind: "page", Query: key, Revision: "rev", After: "cable-1"}
	raw := signVisualToken(token)
	got, e := readVisualToken(raw)
	if e != nil || got != token {
		t.Fatal("cursor round trip failed")
	}
	tampered := strings.Split(raw, ".")
	tampered[0] = "e30"
	if _, e = readVisualToken(strings.Join(tampered, ".")); e == nil {
		t.Fatal("tampered cursor accepted")
	}
	scope.PermissionVersion++
	if key == visualQueryKey(scope, "subgraph", q) {
		t.Fatal("permission revision not bound")
	}
	scope.PermissionVersion--
	scope.UserID = "other"
	if key == visualQueryKey(scope, "subgraph", q) {
		t.Fatal("user not bound")
	}
	scope.UserID = "user"
	scope.Projects = []string{"a", "b"}
	if key != visualQueryKey(scope, "subgraph", q) {
		t.Fatal("scope order should be canonical")
	}
	q.Set("q", "other-device")
	if key == visualQueryKey(scope, "subgraph", q) {
		t.Fatal("query filter not bound")
	}
}
func TestVisualQueryLimitsAndUnsupportedHistory(t *testing.T) {
	for _, raw := range []string{"0", "201", "bad", "-1"} {
		if _, e := visualLimit(url.Values{"limit": {raw}}); e == nil {
			t.Fatal("unbounded query accepted")
		}
	}
	if _, e := visualLimit(url.Values{"limit": {"200"}}); e != nil {
		t.Fatal(e)
	}
	s := Service{}
	if _, _, _, e := s.visualBegin(context.Background(), Scope{}, "subgraph", url.Values{"at": {"2026-01-01"}}); e == nil {
		t.Fatal("historical query silently fell back to current data")
	}
	if _, e := s.VisualTrace(context.Background(), Scope{}, VisualTraceInput{Mode: "logical", From: "p"}); e == nil {
		t.Fatal("logical trace without FIB evidence accepted")
	}
}
