package shor_test

import (
	"context"
	"testing"

	"github.com/splch/goqu/algorithm/shor"
)

func TestShor_15(t *testing.T) {
	cfg := shor.Config{N: 15, Shots: 2048}
	res, err := shor.Run(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}

	a, b := res.Factors[0], res.Factors[1]
	if a*b != 15 {
		t.Errorf("factors %d * %d != 15", a, b)
	}
	if a == 1 || b == 1 {
		t.Errorf("trivial factors: %d, %d", a, b)
	}
}

func TestShor_21(t *testing.T) {
	cfg := shor.Config{N: 21, Shots: 2048}
	res, err := shor.Run(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}

	a, b := res.Factors[0], res.Factors[1]
	if a*b != 21 {
		t.Errorf("factors %d * %d != 21", a, b)
	}
}

func TestShor_Errors(t *testing.T) {
	_, err := shor.Run(context.Background(), shor.Config{N: 1})
	if err == nil {
		t.Error("expected error for N=1")
	}

	_, err = shor.Run(context.Background(), shor.Config{N: 4})
	if err == nil {
		t.Error("expected error for prime power N=4")
	}

	_, err = shor.Run(context.Background(), shor.Config{N: 7})
	if err == nil {
		t.Error("expected error for prime N=7")
	}
}

func TestShor_Even(t *testing.T) {
	res, err := shor.Run(context.Background(), shor.Config{N: 6})
	if err != nil {
		t.Fatal(err)
	}
	a, b := res.Factors[0], res.Factors[1]
	if a*b != 6 {
		t.Errorf("factors %d * %d != 6", a, b)
	}
}
