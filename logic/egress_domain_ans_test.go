package logic

import (
	"slices"
	"testing"

	"github.com/gravitl/netmaker/schema"
)

func TestSetEgressDomainAnsForDomain_mergesPerDomain(t *testing.T) {
	var e schema.Egress
	SetEgressDomainAnsForDomain(&e, "a.example.com", []string{"1.2.3.4/32"})
	SetEgressDomainAnsForDomain(&e, "b.example.com", []string{"5.6.7.8/32"})

	m := DomainAnsMapFromEgress(e)
	if !slices.Equal(m["a.example.com"], []string{"1.2.3.4/32"}) {
		t.Fatalf("a.example.com got %v", m["a.example.com"])
	}
	if !slices.Equal(m["b.example.com"], []string{"5.6.7.8/32"}) {
		t.Fatalf("b.example.com got %v", m["b.example.com"])
	}
	flat := AllDomainAnsFromEgress(e)
	if len(flat) != 2 {
		t.Fatalf("flatten got %v", flat)
	}
}

func TestClearEgressDomainAns(t *testing.T) {
	var e schema.Egress
	SetEgressDomainAnsForDomain(&e, "a.example.com", []string{"1.2.3.4/32"})
	ClearEgressDomainAns(&e)
	if HasEgressDomainAns(e) {
		t.Fatal("expected cleared")
	}
}
