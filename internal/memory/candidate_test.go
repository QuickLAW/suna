package memory

import "testing"

func TestExtractCandidateKeepsOnlyDurableUserProfileSignals(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want bool
	}{
		{name: "explicit preference", in: "以后回复先给我简短结论", want: true},
		{name: "trivial", in: "可行", want: false},
		{name: "task request", in: "我希望你帮我修复这个 bug", want: false},
		{name: "path detail", in: "看 /tmp/project/config.toml", want: false},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			_, got := ExtractCandidate(tt.in, false)
			if got != tt.want {
				t.Fatalf("ExtractCandidate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExtractCandidateClassifiesNegativeFutureInstructionAsConstraint(t *testing.T) {
	candidate, ok := ExtractCandidate("以后不要再没看代码就下结论", false)
	if !ok {
		t.Fatal("ExtractCandidate() did not return candidate")
	}
	if candidate.Kind != MemoryKindConstraint {
		t.Fatalf("candidate.Kind = %q, want %q", candidate.Kind, MemoryKindConstraint)
	}
}

func TestNormalizeTagsDropsSpecificAndUnsafeTags(t *testing.T) {
	got := normalizeTags([]string{"Coding", "foo/bar", "https://example.com", "internal.go", "debugging", "coding"})
	want := []string{"coding", "debugging"}
	if len(got) != len(want) {
		t.Fatalf("normalizeTags() = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("normalizeTags()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}
