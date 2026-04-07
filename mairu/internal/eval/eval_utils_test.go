package eval

import (
	"math"
	"testing"
)

var (
	resultXY = []RetrievalResult{{ID: "x"}, {ID: "y"}}
	resultYX = []RetrievalResult{{ID: "y"}, {ID: "x"}}
)

// --- MRR ---

func TestMRR_FirstHit(t *testing.T) {
	if got := MeanReciprocalRank([]string{"x"}, resultXY); got != 1.0 {
		t.Fatalf("expected 1.0 got %f", got)
	}
}

func TestMRR_SecondHit(t *testing.T) {
	if got := MeanReciprocalRank([]string{"y"}, resultXY); got != 0.5 {
		t.Fatalf("expected 0.5 got %f", got)
	}
}

func TestMRR_NoHit(t *testing.T) {
	if got := MeanReciprocalRank([]string{"z"}, resultXY); got != 0 {
		t.Fatalf("expected 0 got %f", got)
	}
}

// --- Recall@K ---

func TestRecallAtK_Partial(t *testing.T) {
	if got := RecallAtK([]string{"x", "z"}, resultXY, 1); got != 0.5 {
		t.Fatalf("expected 0.5 got %f", got)
	}
}

func TestRecallAtK_Full(t *testing.T) {
	if got := RecallAtK([]string{"x", "y"}, resultXY, 2); got != 1.0 {
		t.Fatalf("expected 1.0 got %f", got)
	}
}

func TestRecallAtK_NoneRelevant(t *testing.T) {
	if got := RecallAtK([]string{"z"}, resultXY, 5); got != 0 {
		t.Fatalf("expected 0 got %f", got)
	}
}

// --- Precision@K ---

func TestPrecisionAtK_AllRelevant(t *testing.T) {
	if got := PrecisionAtK([]string{"x", "y"}, resultXY, 2); got != 1.0 {
		t.Fatalf("expected 1.0 got %f", got)
	}
}

func TestPrecisionAtK_HalfRelevant(t *testing.T) {
	// top-2 results are [x, y]; only "x" is relevant → 1/2
	if got := PrecisionAtK([]string{"x"}, resultXY, 2); got != 0.5 {
		t.Fatalf("expected 0.5 got %f", got)
	}
}

func TestPrecisionAtK_NoneRelevant(t *testing.T) {
	if got := PrecisionAtK([]string{"z"}, resultXY, 2); got != 0 {
		t.Fatalf("expected 0 got %f", got)
	}
}

// --- NDCG@K ---

func TestNDCGAtK_PerfectRanking(t *testing.T) {
	// Both relevant docs are at ranks 1 and 2 → NDCG = 1.0
	if got := NDCGAtK([]string{"x", "y"}, resultXY, 2); math.Abs(got-1.0) > 1e-9 {
		t.Fatalf("expected 1.0 got %f", got)
	}
}

func TestNDCGAtK_ReversedRanking(t *testing.T) {
	// Relevant docs at same positions but swapped; NDCG should still be 1.0
	// because binary relevance DCG is symmetric for equally-weighted docs.
	if got := NDCGAtK([]string{"x", "y"}, resultYX, 2); math.Abs(got-1.0) > 1e-9 {
		t.Fatalf("expected 1.0 got %f", got)
	}
}

func TestNDCGAtK_SecondPositionOnly(t *testing.T) {
	// Only "y" is relevant and it's at rank 2.
	// DCG  = 1/log2(3) ≈ 0.631
	// IDCG = 1/log2(2) = 1.0
	// NDCG = 0.631
	expected := 1.0 / math.Log2(3)
	if got := NDCGAtK([]string{"y"}, resultXY, 2); math.Abs(got-expected) > 1e-6 {
		t.Fatalf("expected %f got %f", expected, got)
	}
}

func TestNDCGAtK_NoRelevant(t *testing.T) {
	if got := NDCGAtK([]string{"z"}, resultXY, 2); got != 0 {
		t.Fatalf("expected 0 got %f", got)
	}
}

// --- AveragePrecision / MAP ---

func TestAveragePrecision_PerfectOrder(t *testing.T) {
	// Both docs relevant; at rank 1 precision=1.0, rank 2 precision=1.0
	// AP = (1.0 + 1.0) / 2 = 1.0
	if got := AveragePrecision([]string{"x", "y"}, resultXY); math.Abs(got-1.0) > 1e-9 {
		t.Fatalf("expected 1.0 got %f", got)
	}
}

func TestAveragePrecision_SecondOnly(t *testing.T) {
	// "y" is relevant at rank 2 → precision at rank 2 = 1/2; AP = 0.5/1 = 0.5
	if got := AveragePrecision([]string{"y"}, resultXY); math.Abs(got-0.5) > 1e-9 {
		t.Fatalf("expected 0.5 got %f", got)
	}
}

func TestAveragePrecision_NoRelevant(t *testing.T) {
	if got := AveragePrecision([]string{"z"}, resultXY); got != 0 {
		t.Fatalf("expected 0 got %f", got)
	}
}

// --- EvaluateCases integration ---

func TestEvaluateCases_AllMetricsComputed(t *testing.T) {
	cases := []Case{
		{Expected: []string{"x"}, Got: resultXY},
		{Expected: []string{"y"}, Got: resultXY},
	}
	m := EvaluateCases(cases, 2)
	if m.MRR == 0 {
		t.Error("MRR should be non-zero")
	}
	if m.NDCG == 0 {
		t.Error("NDCG should be non-zero")
	}
	if m.MAP == 0 {
		t.Error("MAP should be non-zero")
	}
	if m.Precision == 0 {
		t.Error("Precision should be non-zero")
	}
}

func TestEvaluateCases_NegativeCase_ZeroScores(t *testing.T) {
	// A "negative" test case: no expected docs appear in results.
	cases := []Case{
		{Expected: []string{"missing"}, Got: resultXY},
	}
	m := EvaluateCases(cases, 2)
	if m.MRR != 0 || m.Recall != 0 || m.NDCG != 0 || m.MAP != 0 {
		t.Errorf("all metrics should be 0 for negative case, got %+v", m)
	}
}
