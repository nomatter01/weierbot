package main

import (
	"fmt"
	"math/rand"
)

type second_data struct {
	singular  string
	plural    string
	something bool
}

var (
	start = []string{
		"Just biject it to a",
		"Just view the problem as a",
	}
	first = []string{
		"abelian",
		"associative",
		"computable",
		"Lebesgue-measurable",
		"semi-decidable",
		"simple",
		"combinatorial",
		"structure-preserving",
		"diagonalizable",
		"nonsingular",
		"orientable",
		"twice-differentiable",
		"thrice-differentiable",
		"countable",
		"prime",
		"complete",
		"continuous",
		"trivial",
		"3-connected",
		"bipartite",
		"planar",
		"finite",
		"nondeterministic",
		"alternating",
		"convex",
		"undecidable",
		"dihedral",
		"context-free",
		"rational",
		"regular",
		"Noetherian",
		"Cauchy",
		"open",
		"closed",
		"compact",
		"clopen",
		"pointless",
	}
	second = []second_data{
		{"multiset", "multisets", true},
		{"integer", "integers", false},
		{"metric space", "metric spaces", true},
		{"group", "groups", true},
		{"monoid", "monoids", true},
		{"semigroup", "semigroups", true},
		{"ring", "rings", true},
		{"field", "fields", true},
		{"module", "modules", true},
		{"Turing machine", "Turing machines", false},
		{"topological space", "topological spaces", true},
		{"automorphism", "automorphisms", false},
		{"bijection", "bijections", false},
		{"DAG", "DAGs", false},
		{"generating function", "generating functions", false},
		{"taylor series", "taylor series", false},
		{"Hilbert space", "Hilbert spaces", true},
		{"linear transformation", "linear transformations", false},
		{"manifold", "manifolds", true},
		{"hypergraph", "hypergraphs", true},
		{"pushdown automaton", "pushdown automata", false},
		{"combinatorial game", "combinatorial games", false},
		{"residue class", "residue classes", true},
		{"equivalence relation", "equivalence relations", false},
		{"logistic system", "logistic systems", true},
		{"tournament", "tournaments", false},
		{"random variable", "random variables", false},
		{"complexity class", "complexity classes", true},
		{"triangulation", "triangulations", false},
		{"unbounded-fan-in circuit", "unbounded-fan-in circuits", false},
		{"log-space reduction", "log-space reductions", false},
		{"language", "languages", true},
		{"poset", "posets", true},
		{"algebra", "algebras", true},
		{"Markov chain", "Markov chains", false},
		{"4-form", "4-forms", false},
		{"7-chain", "7-chains", false},
	}

	suffix = map[bool]string{
		true:  "n",
		false: "",
	}
)

func addn(ind int) bool {
	return first[ind][0] == 'a' ||
		first[ind][0] == 'e' ||
		first[ind][0] == 'i' ||
		first[ind][0] == 'o' ||
		first[ind][0] == 'u'
}

func randomStart() string {
	var ind = rand.Intn(len(start))
	return start[ind]
}

func randomFirst() string {
	var ind = rand.Intn(len(first))
	return first[ind]
}

func randomSecond(plural bool) string {
	var ind = rand.Intn(len(start))
	if plural {
		return second[ind].plural
	} else {
		for !second[ind].something {
			ind = rand.Intn(len(second))
		}
		return second[ind].singular
	}
	return ""
}

func buildProof() string {
	firstInd := rand.Intn(len(first))

	str := suffix[addn(firstInd)]

	text := fmt.Sprintf("The proof is trivial! %s%s %s %s whose elements are %s %s.", randomStart(), str, first[firstInd], randomSecond(false),
		randomFirst(), randomSecond(true))
	return text
}
