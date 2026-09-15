package quantity

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Parse
// ---------------------------------------------------------------------------

func TestParse_ValidInputs(t *testing.T) {
	tests := []struct {
		input     string
		wantBytes int64
	}{
		// ── Space-separated scalar and unit ─────────────────────────────────────
		{"1 Ki", 1 << 10},
		{"512 Mi", 512 * (1 << 20)},
		{"10 Gi", 10 * (1 << 30)},
		{"2 Ti", 2 * (1 << 40)},
		{"4 Pi", 4 * (1 << 50)},
		{"1 Ei", 1 << 60},
		{"0 Gi", 0},
		// ── Ki ──────────────────────────────────────────────────────────────
		{"1Ki", 1 << 10},
		{"512Ki", 512 * (1 << 10)},
		{"1024Ki", 1024 * (1 << 10)},

		// ── Mi ──────────────────────────────────────────────────────────────
		{"1Mi", 1 << 20},
		{"256Mi", 256 * (1 << 20)},
		{"512Mi", 512 * (1 << 20)},
		{"1024Mi", 1024 * (1 << 20)},

		// ── Gi ──────────────────────────────────────────────────────────────
		{"1Gi", 1 << 30},
		{"2Gi", 2 * (1 << 30)},
		{"10Gi", 10 * (1 << 30)},
		{"64Gi", 64 * (1 << 30)},

		// ── Ti ──────────────────────────────────────────────────────────────
		{"1Ti", 1 << 40},
		{"2Ti", 2 * (1 << 40)},
		{"10Ti", 10 * (1 << 40)},

		// ── Pi ──────────────────────────────────────────────────────────────
		{"1Pi", 1 << 50},
		{"4Pi", 4 * (1 << 50)},

		// ── Ei ──────────────────────────────────────────────────────────────
		{"1Ei", 1 << 60},
		{"2Ei", 2 * (1 << 60)},

		// ── Boundary: zero value ─────────────────────────────────────────────
		{"0Ki", 0},
		{"0Mi", 0},
		{"0Gi", 0},
		{"0Ti", 0},
		{"0Pi", 0},
		{"0Ei", 0},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.input, func(t *testing.T) {
			q, err := Parse(tc.input)
			require.NoError(t, err)
			assert.Equal(t, tc.wantBytes, q.Bytes())
			assert.Equal(t, tc.input, q.String())
		})
	}
}

func TestParse_InvalidInputs(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		// ── Wrong / missing suffix ───────────────────────────────────────────
		{name: "no suffix", input: "1024"},
		{name: "SI suffix MB", input: "512MB"},
		{name: "SI suffix GB", input: "10GB"},
		{name: "SI suffix TB", input: "1TB"},
		{name: "long IEC suffix MiB", input: "512MiB"},
		{name: "long IEC suffix GiB", input: "10GiB"},
		{name: "long IEC suffix TiB", input: "1TiB"},
		{name: "lowercase suffix mi", input: "512mi"},
		{name: "lowercase suffix gi", input: "10gi"},
		{name: "mixed case suffix mI", input: "512mI"},

		// ── Invalid numeric part ─────────────────────────────────────────────
		{name: "float with dot", input: "1.5Gi"},
		{name: "negative value", input: "-1Gi"},
		{name: "leading plus sign", input: "+1Gi"},
		{name: "hex value", input: "0xFFGi"},
		{name: "empty numeric part", input: "Gi"},
		// ── Completely invalid strings ───────────────────────────────────────
		{name: "empty string", input: ""},
		{name: "only suffix", input: "Mi"},
		{name: "only whitespace", input: "   "},
		{name: "random string", input: "abc"},
		{name: "suffix only uppercase", input: "GI"},
		{name: "number with newline", input: "1\nGi"},
		{name: "number with tab", input: "1\tGi"},

		// ── Unsupported suffixes ─────────────────────────────────────────────
		{name: "Zi suffix (beyond Ei)", input: "1Zi"},
		{name: "Yi suffix (beyond Ei)", input: "1Yi"},
		{name: "B suffix (plain bytes label)", input: "1024B"},
		{name: "K suffix (SI kilo)", input: "1K"},
		{name: "M suffix (SI mega)", input: "1M"},
		{name: "G suffix (SI giga)", input: "1G"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			q, err := Parse(tc.input)
			require.Error(t, err, "expected error for input %q", tc.input)
			assert.Zero(t, q.Bytes())
			assert.Empty(t, q.String())
		})
	}
}

// ---------------------------------------------------------------------------
// Quantity.Bytes
// ---------------------------------------------------------------------------

func TestQuantity_Bytes(t *testing.T) {
	tests := []struct {
		input     string
		wantBytes int64
	}{
		{"1Ki", 1024},
		{"1Mi", 1048576},
		{"1Gi", 1073741824},
		{"1Ti", 1099511627776},
		{"1Pi", 1125899906842624},
		{"1Ei", 1152921504606846976},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.input, func(t *testing.T) {
			q, err := Parse(tc.input)
			require.NoError(t, err)
			assert.Equal(t, tc.wantBytes, q.Bytes())
		})
	}
}

// ---------------------------------------------------------------------------
// Quantity.String
// ---------------------------------------------------------------------------

func TestQuantity_String(t *testing.T) {
	inputs := []string{
		// original compact forms
		"1Ki", "512Mi", "10Gi", "2Ti", "4Pi", "1Ei",
		// space-separated forms — String() must return the original input exactly
		"1 Ki", "512 Mi", "10 Gi", "2 Ti", "4 Pi", "1 Ei",
	}

	for _, input := range inputs {
		input := input
		t.Run(input, func(t *testing.T) {
			q, err := Parse(input)
			require.NoError(t, err)
			assert.Equal(t, input, q.String())
		})
	}
}

// ---------------------------------------------------------------------------
// Quantity.Cmp
// ---------------------------------------------------------------------------

func TestQuantity_Cmp(t *testing.T) {
	tests := []struct {
		name    string
		a       string
		b       string
		wantCmp int
	}{
		// ── Equal values ─────────────────────────────────────────────────────
		{name: "equal: 1Gi == 1Gi", a: "1Gi", b: "1Gi", wantCmp: 0},
		{name: "equal: 1024Mi == 1Gi", a: "1024Mi", b: "1Gi", wantCmp: 0},
		{name: "equal: 1024Ki == 1Mi", a: "1024Ki", b: "1Mi", wantCmp: 0},
		{name: "equal: 1024Gi == 1Ti", a: "1024Gi", b: "1Ti", wantCmp: 0},
		{name: "equal: 0Ki == 0Mi", a: "0Ki", b: "0Mi", wantCmp: 0},

		// ── a < b ────────────────────────────────────────────────────────────
		{name: "less: 512Mi < 1Gi", a: "512Mi", b: "1Gi", wantCmp: -1},
		{name: "less: 1Ki < 1Mi", a: "1Ki", b: "1Mi", wantCmp: -1},
		{name: "less: 1Gi < 1Ti", a: "1Gi", b: "1Ti", wantCmp: -1},
		{name: "less: 1Ti < 1Pi", a: "1Ti", b: "1Pi", wantCmp: -1},
		{name: "less: 1Pi < 1Ei", a: "1Pi", b: "1Ei", wantCmp: -1},
		{name: "less: 0Ki < 1Ki", a: "0Ki", b: "1Ki", wantCmp: -1},

		// ── a > b ────────────────────────────────────────────────────────────
		{name: "greater: 1Gi > 512Mi", a: "1Gi", b: "512Mi", wantCmp: 1},
		{name: "greater: 1Mi > 1Ki", a: "1Mi", b: "1Ki", wantCmp: 1},
		{name: "greater: 1Ti > 1Gi", a: "1Ti", b: "1Gi", wantCmp: 1},
		{name: "greater: 1Pi > 1Ti", a: "1Pi", b: "1Ti", wantCmp: 1},
		{name: "greater: 1Ei > 1Pi", a: "1Ei", b: "1Pi", wantCmp: 1},
		{name: "greater: 2Gi > 1Gi", a: "2Gi", b: "1Gi", wantCmp: 1},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			a, err := Parse(tc.a)
			require.NoError(t, err)
			b, err := Parse(tc.b)
			require.NoError(t, err)
			assert.Equal(t, tc.wantCmp, a.Cmp(b))
		})
	}
}

func TestQuantity_Cmp_Symmetry(t *testing.T) {
	// If a.Cmp(b) == -1 then b.Cmp(a) must == 1, and vice versa.
	pairs := [][2]string{
		{"512Mi", "1Gi"},
		{"1Ki", "1Mi"},
		{"1Gi", "1Ti"},
	}

	for _, pair := range pairs {
		a, err := Parse(pair[0])
		require.NoError(t, err)
		b, err := Parse(pair[1])
		require.NoError(t, err)

		assert.Equal(t, -1, a.Cmp(b), "%s < %s", pair[0], pair[1])
		assert.Equal(t, 1, b.Cmp(a), "%s > %s", pair[1], pair[0])
	}
}

// ---------------------------------------------------------------------------
// Quantity.AtLeast
// ---------------------------------------------------------------------------

func TestQuantity_AtLeast(t *testing.T) {
	tests := []struct {
		name        string
		actual      string
		required    string
		wantAtLeast bool
	}{
		// ── actual > required ────────────────────────────────────────────────
		{name: "2Gi >= 1Gi → true", actual: "2Gi", required: "1Gi", wantAtLeast: true},
		{name: "1Gi >= 512Mi → true", actual: "1Gi", required: "512Mi", wantAtLeast: true},
		{name: "1Ti >= 1Gi → true", actual: "1Ti", required: "1Gi", wantAtLeast: true},
		{name: "1Ei >= 1Pi → true", actual: "1Ei", required: "1Pi", wantAtLeast: true},

		// ── actual == required ───────────────────────────────────────────────
		{name: "1Gi >= 1Gi → true (equal)", actual: "1Gi", required: "1Gi", wantAtLeast: true},
		{
			name:        "512Mi >= 512Mi → true (equal)",
			actual:      "512Mi",
			required:    "512Mi",
			wantAtLeast: true,
		},
		{
			name:        "1024Mi >= 1Gi → true (cross-suffix equal)",
			actual:      "1024Mi",
			required:    "1Gi",
			wantAtLeast: true,
		},
		{name: "0Ki >= 0Mi → true (both zero)", actual: "0Ki", required: "0Mi", wantAtLeast: true},

		// ── actual < required ────────────────────────────────────────────────
		{name: "512Mi >= 1Gi → false", actual: "512Mi", required: "1Gi", wantAtLeast: false},
		{name: "1Gi >= 2Gi → false", actual: "1Gi", required: "2Gi", wantAtLeast: false},
		{name: "1Ki >= 1Mi → false", actual: "1Ki", required: "1Mi", wantAtLeast: false},
		{name: "1Pi >= 1Ei → false", actual: "1Pi", required: "1Ei", wantAtLeast: false},
		{name: "0Ki >= 1Ki → false", actual: "0Ki", required: "1Ki", wantAtLeast: false},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			actual, err := Parse(tc.actual)
			require.NoError(t, err)
			required, err := Parse(tc.required)
			require.NoError(t, err)
			assert.Equal(t, tc.wantAtLeast, actual.AtLeast(required))
		})
	}
}

// ---------------------------------------------------------------------------
// Multiplier correctness — each suffix independently verified
// ---------------------------------------------------------------------------

func TestParse_MultiplierCorrectness(t *testing.T) {
	// Verifies the exact byte value for 1 unit of each suffix.
	tests := []struct {
		suffix    string
		wantBytes int64
	}{
		{"Ki", 1_024},
		{"Mi", 1_048_576},
		{"Gi", 1_073_741_824},
		{"Ti", 1_099_511_627_776},
		{"Pi", 1_125_899_906_842_624},
		{"Ei", 1_152_921_504_606_846_976},
	}

	for _, tc := range tests {
		tc := tc
		t.Run("1"+tc.suffix, func(t *testing.T) {
			q, err := Parse("1" + tc.suffix)
			require.NoError(t, err)
			assert.Equal(t, tc.wantBytes, q.Bytes())
		})
	}
}

// ---------------------------------------------------------------------------
// Cross-suffix equivalence
// ---------------------------------------------------------------------------

func TestParse_CrossSuffixEquivalence(t *testing.T) {
	// Quantities that are mathematically equal across different suffixes
	// must produce identical byte counts.
	pairs := []struct {
		name string
		a    string
		b    string
	}{
		{name: "1024Ki == 1Mi", a: "1024Ki", b: "1Mi"},
		{name: "1024Mi == 1Gi", a: "1024Mi", b: "1Gi"},
		{name: "1024Gi == 1Ti", a: "1024Gi", b: "1Ti"},
		{name: "1024Ti == 1Pi", a: "1024Ti", b: "1Pi"},
		{name: "1024Pi == 1Ei", a: "1024Pi", b: "1Ei"},
	}

	for _, tc := range pairs {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			a, err := Parse(tc.a)
			require.NoError(t, err)
			b, err := Parse(tc.b)
			require.NoError(t, err)
			assert.Equal(t, a.Bytes(), b.Bytes())
			assert.Equal(t, 0, a.Cmp(b))
			assert.True(t, a.AtLeast(b))
			assert.True(t, b.AtLeast(a))
		})
	}
}

// ---------------------------------------------------------------------------
// Large values — no int64 overflow for valid inputs
// ---------------------------------------------------------------------------

func TestParse_LargeValues_NoOverflow(t *testing.T) {
	tests := []struct {
		input     string
		wantBytes int64
	}{
		// max safe Ei value before int64 overflow: 7Ei = 7 * 2^60
		{"7Ei", 7 * (1 << 60)},
		// large Pi
		{"999Pi", 999 * (1 << 50)},
		// large Ti
		{"9999Ti", 9999 * (1 << 40)},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.input, func(t *testing.T) {
			q, err := Parse(tc.input)
			require.NoError(t, err)
			assert.Equal(t, tc.wantBytes, q.Bytes())
		})
	}
}
