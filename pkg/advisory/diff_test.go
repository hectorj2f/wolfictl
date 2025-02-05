package advisory

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	v2 "github.com/wolfi-dev/wolfictl/pkg/configs/advisory/v2"
	rwos "github.com/wolfi-dev/wolfictl/pkg/configs/rwfs/os"
)

var (
	unixEpochTimestamp         = v2.Timestamp(time.Unix(0, 0))
	unixEpochTimestampPlus1Day = v2.Timestamp(time.Unix(0, 0).AddDate(0, 0, 1))

	// now establishes a fixed time for testing recency validation, for deterministic test runs.
	now = time.Unix(1699660800, 0) // Nov 11 2023 00:00:00 UTC
)

func TestIndexDiff(t *testing.T) {
	cases := []struct {
		name               string
		expectedDiffResult IndexDiffResult
	}{
		{
			name:               "same",
			expectedDiffResult: IndexDiffResult{},
		},
		{
			name: "added-document",
			expectedDiffResult: IndexDiffResult{
				Added: []v2.Document{
					{
						SchemaVersion: "2.0.1",
						Package: v2.Package{
							Name: "ko",
						},
						Advisories: v2.Advisories{
							{
								ID: "CVE-2023-24535",
								Events: []v2.Event{
									{
										Timestamp: v2.Timestamp(now),
										Type:      v2.EventTypeTruePositiveDetermination,
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "removed-document",
			expectedDiffResult: IndexDiffResult{
				Removed: []v2.Document{
					{
						SchemaVersion: "2.0.1",
						Package: v2.Package{
							Name: "ko",
						},
						Advisories: v2.Advisories{
							{
								ID: "CVE-2023-24535",
								Events: []v2.Event{
									{
										Timestamp: unixEpochTimestamp,
										Type:      v2.EventTypeTruePositiveDetermination,
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "added-advisory",
			expectedDiffResult: IndexDiffResult{
				Modified: []DocumentDiffResult{
					{
						Name: "ko",
						Added: v2.Advisories{
							{
								ID: "CVE-2023-11111",
								Events: []v2.Event{
									{
										Timestamp: v2.Timestamp(now),
										Type:      v2.EventTypeTruePositiveDetermination,
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "removed-advisory",
			expectedDiffResult: IndexDiffResult{
				Modified: []DocumentDiffResult{
					{
						Name: "ko",
						Removed: v2.Advisories{
							{
								ID: "CVE-2023-11111",
								Events: []v2.Event{
									{
										Timestamp: unixEpochTimestamp,
										Type:      v2.EventTypeTruePositiveDetermination,
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "modified-advisory-outside-of-events",
			expectedDiffResult: IndexDiffResult{
				Modified: []DocumentDiffResult{
					{
						Name: "ko",
						Modified: []DiffResult{
							{
								ID: "CVE-2023-24535",
								Added: v2.Advisory{
									ID: "CVE-2023-24535",
									Events: []v2.Event{
										{
											Timestamp: unixEpochTimestamp,
											Type:      v2.EventTypeTruePositiveDetermination,
										},
									},
								},
								Removed: v2.Advisory{
									ID: "CVE-2023-24535",
									Aliases: []string{
										"GHSA-2222-2222-2222",
									},
									Events: []v2.Event{
										{
											Timestamp: unixEpochTimestamp,
											Type:      v2.EventTypeTruePositiveDetermination,
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "added-event",
			expectedDiffResult: IndexDiffResult{
				Modified: []DocumentDiffResult{
					{
						Name: "ko",
						Modified: []DiffResult{
							{
								ID: "CVE-2023-11111",
								Added: v2.Advisory{
									ID: "CVE-2023-11111",
									Events: []v2.Event{
										{
											Timestamp: unixEpochTimestamp,
											Type:      v2.EventTypeTruePositiveDetermination,
										},
										{
											Timestamp: v2.Timestamp(now),
											Type:      v2.EventTypeTruePositiveDetermination,
										},
									},
								},
								Removed: v2.Advisory{
									ID: "CVE-2023-11111",
									Events: []v2.Event{
										{
											Timestamp: unixEpochTimestamp,
											Type:      v2.EventTypeTruePositiveDetermination,
										},
									},
								},
								AddedEvents: []v2.Event{
									{
										Timestamp: v2.Timestamp(now),
										Type:      v2.EventTypeTruePositiveDetermination,
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "removed-event",
			expectedDiffResult: IndexDiffResult{
				Modified: []DocumentDiffResult{
					{
						Name: "ko",
						Modified: []DiffResult{
							{
								ID: "CVE-2023-11111",
								Added: v2.Advisory{
									ID: "CVE-2023-11111",
									Events: []v2.Event{
										{
											Timestamp: unixEpochTimestamp,
											Type:      v2.EventTypeTruePositiveDetermination,
										},
									},
								},
								Removed: v2.Advisory{
									ID: "CVE-2023-11111",
									Events: []v2.Event{
										{
											Timestamp: unixEpochTimestamp,
											Type:      v2.EventTypeTruePositiveDetermination,
										},
										{
											Timestamp: unixEpochTimestampPlus1Day,
											Type:      v2.EventTypeFalsePositiveDetermination,
										},
									},
								},
								RemovedEvents: []v2.Event{
									{
										Timestamp: unixEpochTimestampPlus1Day,
										Type:      v2.EventTypeFalsePositiveDetermination,
									},
								},
							},
						},
					},
				},
			},
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			aDir := filepath.Join("testdata", "diff", tt.name, "a")
			bDir := filepath.Join("testdata", "diff", tt.name, "b")
			aFsys := rwos.DirFS(aDir)
			bFsys := rwos.DirFS(bDir)
			aIndex, err := v2.NewIndex(aFsys)
			require.NoError(t, err)
			bIndex, err := v2.NewIndex(bFsys)
			require.NoError(t, err)

			diff := IndexDiff(aIndex, bIndex)

			if d := cmp.Diff(tt.expectedDiffResult, diff); d != "" {
				t.Errorf("unexpected diff result (-want +got):\n%s", d)
			}
		})
	}
}
