package domain

import "testing"

func TestDerivePostStatus(t *testing.T) {
	cases := []struct {
		name    string
		targets []TargetStatus
		want    PostStatus
	}{
		{"todos publicados", []TargetStatus{TargetStatusPublished, TargetStatusPublished}, PostStatusPublished},
		{"todos falharam", []TargetStatus{TargetStatusFailed, TargetStatusFailed}, PostStatusFailed},
		{"parcial (o caso comum)", []TargetStatus{TargetStatusPublished, TargetStatusFailed}, PostStatusPartial},
		{"agendado para o futuro", []TargetStatus{TargetStatusScheduled, TargetStatusScheduled}, PostStatusScheduled},
		{"fan-out em andamento", []TargetStatus{TargetStatusPending, TargetStatusPending}, PostStatusPublishing},
		{"sem targets", nil, PostStatusScheduled},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := DerivePostStatus(tc.targets); got != tc.want {
				t.Fatalf("DerivePostStatus(%v) = %s, esperado %s", tc.targets, got, tc.want)
			}
		})
	}
}
