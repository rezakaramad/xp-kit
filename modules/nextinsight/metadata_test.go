package nextinsight

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// normalize
// ---------------------------------------------------------------------------

func TestNormalize(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		// Basic lowercasing
		{"Production", "production"},
		// Spaces become hyphens
		{"In House", "in-house"},
		// Underscores become hyphens
		{"in_house", "in-house"},
		// Slashes become hyphens
		{"buy/build", "buy-build"},
		// Mixed spaces, underscores, slashes
		{"My App/Name_v2.0", "my-app-name-v2.0"},
		// Leading/trailing hyphens stripped
		{"-leading", "leading"},
		{"trailing-", "trailing"},
		// Special characters dropped
		{"hello@world!", "helloworld"},
		// Multiple consecutive hyphens collapsed
		{"a--b---c", "a-b-c"},
		// Dot preserved
		{"v2.0", "v2.0"},
		// Whitespace trimmed
		{"  trimmed  ", "trimmed"},
		// Empty string
		{"", ""},
		// Already normalised
		{"agile-team", "agile-team"},
		// Exactly 63 chars — unchanged
		{strings.Repeat("a", 63), strings.Repeat("a", 63)},
		// 64 chars — truncated to 63
		{strings.Repeat("a", 64), strings.Repeat("a", 63)},
		// Truncation should not leave trailing hyphen
		{strings.Repeat("a", 62) + "-x", strings.Repeat("a", 62)},
	}

	for _, tc := range cases {
		got := normalize(tc.in)
		if got != tc.want {
			t.Errorf("normalize(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

const testPrefix = "nextinsight.rezakara.demo/"

// ---------------------------------------------------------------------------
// OwnershipMetadata.NamespaceLabels
// ---------------------------------------------------------------------------

func TestNamespaceLabels_ReturnsOnlyOwnershipLabels(t *testing.T) {
	m := &TenantMetadata{
		AgileReleaseTrain: "ART Platform",
		AgileTeam:         "Team Falcon",
	}

	labels := m.TenantLabels(testPrefix)

	if len(labels) != 2 {
		t.Errorf("expected exactly 2 namespace labels, got %d: %v", len(labels), labels)
	}
	if labels[testPrefix+"agile-release-train"] != "art-platform" {
		t.Errorf("agile-release-train = %q, want %q", labels[testPrefix+"agile-release-train"], "art-platform")
	}
	if labels[testPrefix+"agile-team"] != "team-falcon" {
		t.Errorf("agile-team = %q, want %q", labels[testPrefix+"agile-team"], "team-falcon")
	}
	for _, forbidden := range []string{
		testPrefix + "app-id", testPrefix + "app-name",
		testPrefix + "lifecycle", testPrefix + "criticality",
	} {
		if _, ok := labels[forbidden]; ok {
			t.Errorf("namespace labels must not contain workload key %q", forbidden)
		}
	}
}

func TestNamespaceLabels_OmitsEmptyOwnershipFields(t *testing.T) {
	labels := (&TenantMetadata{}).TenantLabels(testPrefix)
	if len(labels) != 0 {
		t.Errorf("expected empty map when ART and team are empty, got %v", labels)
	}
}

// ---------------------------------------------------------------------------
// ApplicationMetadata.WorkloadLabels
// ---------------------------------------------------------------------------

func TestWorkloadLabels_ReturnsOnlyApplicationLabels(t *testing.T) {
	m := &ApplicationMetadata{
		ApplicationID:     "123",
		ApplicationName:   "Platform App",
		Lifecycle:         "Production",
		LifecycleDecision: "Keep",
		Criticality:       "High",
		Complexity:        "Medium",
		Category:          "Infrastructure",
		DevelopmentType:   "In House",
		SourcingType:      "Internal",
		FacingInternet:    "true",
	}

	labels := m.WorkloadLabels(testPrefix)

	expected := map[string]string{
		testPrefix + "app-id":             "123",
		testPrefix + "app-name":           "platform-app",
		testPrefix + "lifecycle":          "production",
		testPrefix + "lifecycle-decision": "keep",
		testPrefix + "criticality":        "high",
		testPrefix + "complexity":         "medium",
		testPrefix + "category":           "infrastructure",
		testPrefix + "development-type":   "in-house",
		testPrefix + "sourcing-type":      "internal",
		testPrefix + "facing-internet":    "true",
	}
	if len(labels) != len(expected) {
		t.Errorf("expected %d workload labels, got %d: %v", len(expected), len(labels), labels)
	}
	for key, want := range expected {
		if got := labels[key]; got != want {
			t.Errorf("label %q = %q, want %q", key, got, want)
		}
	}
	for _, forbidden := range []string{
		testPrefix + "agile-release-train",
		testPrefix + "agile-team",
	} {
		if _, ok := labels[forbidden]; ok {
			t.Errorf("workload labels must not contain namespace key %q", forbidden)
		}
	}
}

func TestWorkloadLabels_OmitsEmptyFields(t *testing.T) {
	m := &ApplicationMetadata{ApplicationID: "42"}
	labels := m.WorkloadLabels(testPrefix)

	if len(labels) != 1 {
		t.Errorf("expected 1 workload label, got %d: %v", len(labels), labels)
	}
	if labels[testPrefix+"app-id"] != "42" {
		t.Errorf("app-id = %q, want %q", labels[testPrefix+"app-id"], "42")
	}
}

func TestWorkloadLabels_EmptyMetadataReturnsEmptyMap(t *testing.T) {
	labels := (&ApplicationMetadata{}).WorkloadLabels(testPrefix)
	if len(labels) != 0 {
		t.Errorf("expected empty map, got %v", labels)
	}
}

// ---------------------------------------------------------------------------
// buildMetadata — first-wins for ART and Agile Team
// ---------------------------------------------------------------------------

func TestBuildOwnershipMetadata_ReturnsTeamAndParent(t *testing.T) {
	group := &groupResponse{}
	group.Data.Name = "Team-Alpha"
	group.Data.GroupType.Name = "Agile Team"
	group.Data.ParentGroup.Name = "ART-One"

	meta := buildOwnershipMetadata(group)

	if meta.AgileReleaseTrain != "ART-One" {
		t.Errorf("AgileReleaseTrain = %q, want %q", meta.AgileReleaseTrain, "ART-One")
	}
	if meta.AgileTeam != "Team-Alpha" {
		t.Errorf("AgileTeam = %q, want %q", meta.AgileTeam, "Team-Alpha")
	}
}

func TestBuildOwnershipMetadata_NoParentGroup(t *testing.T) {
	group := &groupResponse{}
	group.Data.Name = "Team-Alpha"
	group.Data.GroupType.Name = "Agile Team"
	// ParentGroup.Name is empty

	meta := buildOwnershipMetadata(group)

	if meta.AgileTeam != "Team-Alpha" {
		t.Errorf("AgileTeam = %q, want %q", meta.AgileTeam, "Team-Alpha")
	}
	if meta.AgileReleaseTrain != "" {
		t.Errorf("expected empty AgileReleaseTrain, got %q", meta.AgileReleaseTrain)
	}
}

func TestBuildApplicationMetadata_FacingInternetLowercased(t *testing.T) {
	app := &applicationResponse{}
	app.Data.FacingInternet = "TRUE"

	meta := buildApplicationMetadata("3", app)

	if meta.FacingInternet != "true" {
		t.Errorf("FacingInternet = %q, want %q", meta.FacingInternet, "true")
	}
}
