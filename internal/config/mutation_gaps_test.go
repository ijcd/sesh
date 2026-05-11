package config

// Tests written in response to LIVED mutations surfaced by gremlins.
// Each test name references the mutation location.

import (
	"reflect"
	"testing"

	"github.com/ijcd/sesh/internal/spec"
)

// config.go:58 — applyGlobalDefaults Attach propagation.
// Mutations: negating p.Attach==nil or g.Attach!=nil each survived.

func TestApplyGlobalDefaults_AttachPropagatedFromGlobal(t *testing.T) {
	attach := true
	p := &spec.Project{}
	g := &Global{Attach: &attach}
	applyGlobalDefaults(p, g)
	if p.Attach == nil || *p.Attach != true {
		t.Errorf("Attach should be copied from global when project Attach is nil, got %v", p.Attach)
	}
}

func TestApplyGlobalDefaults_AttachNotOverwrittenWhenProjectSetsIt(t *testing.T) {
	proj := false
	global := true
	p := &spec.Project{Attach: &proj}
	g := &Global{Attach: &global}
	applyGlobalDefaults(p, g)
	if *p.Attach != false {
		t.Errorf("existing project Attach=false should not be overwritten by global true, got %v", *p.Attach)
	}
}

func TestApplyGlobalDefaults_AttachNotCopiedWhenGlobalNil(t *testing.T) {
	p := &spec.Project{}
	g := &Global{Attach: nil}
	applyGlobalDefaults(p, g)
	if p.Attach != nil {
		t.Errorf("Attach should remain nil when global Attach is nil, got %v", p.Attach)
	}
}

// config.go:62 — len(g.Vars) > 0 boundary: no global vars → project vars unchanged.

func TestApplyGlobalDefaults_EmptyGlobalVarsLeavesProjectVarsAlone(t *testing.T) {
	p := &spec.Project{Vars: map[string]string{"a": "1"}}
	g := &Global{Vars: map[string]string{}} // empty, not nil
	applyGlobalDefaults(p, g)
	if len(p.Vars) != 1 || p.Vars["a"] != "1" {
		t.Errorf("project vars should be unchanged with empty global vars, got %v", p.Vars)
	}
}

// merge.go:41 — mergeMap nil handling.
// Mutations: negating p==nil or c==nil in the nil guard survived.

func TestMergeMap_BothNil(t *testing.T) {
	got := mergeMap(nil, nil)
	if got != nil {
		t.Errorf("mergeMap(nil, nil) = %v, want nil", got)
	}
}

func TestMergeMap_ParentNil(t *testing.T) {
	c := map[string]string{"x": "1"}
	got := mergeMap(nil, c)
	want := map[string]string{"x": "1"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("mergeMap(nil, c) = %v, want %v", got, want)
	}
}

func TestMergeMap_ChildNil(t *testing.T) {
	p := map[string]string{"y": "2"}
	got := mergeMap(p, nil)
	want := map[string]string{"y": "2"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("mergeMap(p, nil) = %v, want %v", got, want)
	}
}
