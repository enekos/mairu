package walkers

import (
	"bytes"
	"strings"
	"testing"
)

// psql -c "SELECT id, email, firstName, bio FROM users LIMIT 2" output.
const sampleSQL = ` id |       email       | firstName |       bio
----+-------------------+-----------+-----------------
  1 | jane@acme.io      | Jane      | likes hiking
  2 | john@example.org  | John      | NULL
(2 rows)
`

func TestSQL_RedactsKnownColumns(t *testing.T) {
	opts := newTestOpts(t, true)
	var buf bytes.Buffer
	if _, err := SQL(strings.NewReader(sampleSQL), &buf, opts); err != nil {
		t.Fatalf("SQL: %v", err)
	}
	out := buf.String()
	for _, leak := range []string{"jane@acme.io", "john@example.org", "Jane", "John"} {
		if strings.Contains(out, leak) {
			t.Errorf("raw %q leaked in output:\n%s", leak, out)
		}
	}
	if !strings.Contains(out, "(2 rows)") {
		t.Errorf("footer dropped:\n%s", out)
	}
	// IDs (safe column) and the separator structure survive.
	if !strings.Contains(out, "----+") {
		t.Errorf("separator missing:\n%s", out)
	}
	if !strings.Contains(out, " 1 ") || !strings.Contains(out, " 2 ") {
		t.Errorf("id column lost:\n%s", out)
	}
}

func TestSQL_StrictMasksUnknownColumn(t *testing.T) {
	opts := newTestOpts(t, true)
	opts.Strict = true
	var buf bytes.Buffer
	if _, err := SQL(strings.NewReader(sampleSQL), &buf, opts); err != nil {
		t.Fatalf("SQL: %v", err)
	}
	out := buf.String()
	if strings.Contains(out, "likes hiking") {
		t.Errorf("unknown column `bio` passed through in strict mode:\n%s", out)
	}
}

func TestSQL_PermissiveKeepsUnknownColumn(t *testing.T) {
	opts := newTestOpts(t, true)
	opts.Strict = false
	var buf bytes.Buffer
	if _, err := SQL(strings.NewReader(sampleSQL), &buf, opts); err != nil {
		t.Fatalf("SQL: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "likes hiking") {
		t.Errorf("permissive mode should keep `bio`:\n%s", out)
	}
}

func TestSQL_PassesThroughNonTableLines(t *testing.T) {
	opts := newTestOpts(t, true)
	in := `NOTICE:  extra_float_digits set to 3
SELECT id FROM users LIMIT 1;
 id
----
  1
(1 row)
psql (16.4)
`
	var buf bytes.Buffer
	if _, err := SQL(strings.NewReader(in), &buf, opts); err != nil {
		t.Fatalf("SQL: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"NOTICE:", "SELECT id FROM users LIMIT 1;", "psql (16.4)", "(1 row)"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected non-table line %q to pass through, got:\n%s", want, out)
		}
	}
}

func TestSQL_MultipleBlocks(t *testing.T) {
	opts := newTestOpts(t, true)
	in := ` email
----------
 a@b.io
(1 row)

 email
----------
 c@d.io
(1 row)
`
	var buf bytes.Buffer
	if _, err := SQL(strings.NewReader(in), &buf, opts); err != nil {
		t.Fatalf("SQL: %v", err)
	}
	out := buf.String()
	if strings.Contains(out, "a@b.io") || strings.Contains(out, "c@d.io") {
		t.Errorf("multi-block redaction missed a row:\n%s", out)
	}
	if strings.Count(out, "(1 row)") != 2 {
		t.Errorf("expected two footers, got:\n%s", out)
	}
}

func TestSQL_NoFooter(t *testing.T) {
	opts := newTestOpts(t, true)
	in := ` email
----------
 a@b.io
`
	var buf bytes.Buffer
	if _, err := SQL(strings.NewReader(in), &buf, opts); err != nil {
		t.Fatalf("SQL: %v", err)
	}
	if strings.Contains(buf.String(), "a@b.io") {
		t.Errorf("footer-less block missed redaction:\n%s", buf.String())
	}
}

func TestSQL_SingleColumnSeparator(t *testing.T) {
	// `parseSQLSeparator` must accept separators with no `+` (single-column).
	if _, ok := parseSQLSeparator("--------"); !ok {
		t.Fatal("single-column separator rejected")
	}
	if b, ok := parseSQLSeparator("---+---+---"); !ok || len(b) != 2 {
		t.Fatalf("3-column separator: ok=%v boundaries=%v", ok, b)
	}
	if _, ok := parseSQLSeparator(" leading-space"); ok {
		t.Fatal("separator with leading space should be rejected")
	}
	if _, ok := parseSQLSeparator("not a separator"); ok {
		t.Fatal("free text accepted as separator")
	}
}

func TestSQL_FooterRecognition(t *testing.T) {
	for _, ok := range []string{"(1 row)", "(42 rows)", "  (0 rows)"} {
		if !isSQLFooter(ok) {
			t.Errorf("should accept footer %q", ok)
		}
	}
	for _, no := range []string{"(rows)", "(1)", "rows", "(1 row", "1 row)", "(a rows)"} {
		if isSQLFooter(no) {
			t.Errorf("should reject %q", no)
		}
	}
}
