package main

import (
	"encoding/json"
	"testing"
)

func TestExtractAI(t *testing.T) {
	prefixes := []string{"codex", "claude", "gemini"}

	tests := []struct {
		name     string
		win      kittyWindow
		wantAI   string
		wantOK   bool
	}{
		{
			name: "codex via node binary path",
			win: kittyWindow{
				ForegroundProcesses: []foregroundProcess{
					{
						Cmdline: []string{
							"/Users/akira/.local/share/mise/installs/node/24.5.0/lib/node_modules/@openai/codex/vendor/aarch64-apple-darwin/codex/codex",
						},
					},
				},
			},
			wantAI: "codex",
			wantOK: true,
		},
		{
			name: "codex via node script",
			win: kittyWindow{
				ForegroundProcesses: []foregroundProcess{
					{
						Cmdline: []string{
							"node",
							"/Users/akira/.local/share/mise/installs/node/24.5.0/bin/codex",
						},
					},
				},
			},
			wantAI: "codex",
			wantOK: true,
		},
		{
			name: "claude direct binary",
			win: kittyWindow{
				ForegroundProcesses: []foregroundProcess{
					{
						Cmdline: []string{"/usr/local/bin/claude"},
					},
				},
			},
			wantAI: "claude",
			wantOK: true,
		},
		{
			name: "gemini cli",
			win: kittyWindow{
				ForegroundProcesses: []foregroundProcess{
					{
						Cmdline: []string{"gemini", "chat"},
					},
				},
			},
			wantAI: "gemini",
			wantOK: true,
		},
		{
			name: "no match - just node",
			win: kittyWindow{
				ForegroundProcesses: []foregroundProcess{
					{
						Cmdline: []string{"node", "/some/other/script.js"},
					},
				},
			},
			wantAI: "",
			wantOK: false,
		},
		{
			name: "no match - zsh",
			win: kittyWindow{
				ForegroundProcesses: []foregroundProcess{
					{
						Cmdline: []string{"/bin/zsh"},
					},
				},
			},
			wantAI: "",
			wantOK: false,
		},
		{
			name: "empty cmdline",
			win: kittyWindow{
				ForegroundProcesses: []foregroundProcess{
					{Cmdline: []string{}},
				},
			},
			wantAI: "",
			wantOK: false,
		},
		{
			name: "multiple processes - one is codex",
			win: kittyWindow{
				ForegroundProcesses: []foregroundProcess{
					{Cmdline: []string{"node", "/path/to/context7-mcp"}},
					{Cmdline: []string{"python", "/path/to/serena"}},
					{Cmdline: []string{"/path/to/@openai/codex/vendor/codex/codex"}},
				},
			},
			wantAI: "codex",
			wantOK: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotAI, gotOK := extractAI(tt.win, prefixes)
			if gotAI != tt.wantAI || gotOK != tt.wantOK {
				t.Errorf("extractAI() = (%q, %v), want (%q, %v)", gotAI, gotOK, tt.wantAI, tt.wantOK)
			}
		})
	}
}

func TestParseKittyJSON(t *testing.T) {
	// Simulated kitty @ ls output based on user's actual data
	kittyJSON := `[
		{
			"tabs": [
				{
					"id": 1,
					"title": "codex",
					"windows": [
						{
							"id": 10,
							"title": "codex",
							"cwd": "/Users/akira/src/github.com/atani/idea/lazyccg",
							"foreground_processes": [
								{
									"cmdline": ["node", "/Users/akira/.npm/_npx/eea2bd7412d4593b/node_modules/.bin/context7-mcp"],
									"cwd": "/Users/akira/src/github.com/atani/idea/lazyccg",
									"pid": 35783
								},
								{
									"cmdline": ["/Users/akira/.local/share/mise/installs/node/24.5.0/lib/node_modules/@openai/codex/vendor/aarch64-apple-darwin/codex/codex"],
									"cwd": "/Users/akira/src/github.com/atani/idea/lazyccg",
									"pid": 35646
								},
								{
									"cmdline": ["node", "/Users/akira/.local/share/mise/installs/node/24.5.0/bin/codex"],
									"cwd": "/Users/akira/src/github.com/atani/idea/lazyccg",
									"pid": 35645
								}
							]
						}
					]
				},
				{
					"id": 2,
					"title": "lazyccg",
					"windows": [
						{
							"id": 20,
							"title": "lazyccg",
							"cwd": "/Users/akira/src/github.com/atani/idea/lazyccg",
							"foreground_processes": [
								{
									"cmdline": ["go", "run", "./cmd/lazyccg"],
									"cwd": "/Users/akira/src/github.com/atani/idea/lazyccg",
									"pid": 40000
								}
							]
						}
					]
				}
			]
		}
	]`

	var osWindows []kittyOSWindow
	err := json.Unmarshal([]byte(kittyJSON), &osWindows)
	if err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	if len(osWindows) != 1 {
		t.Fatalf("Expected 1 OS window, got %d", len(osWindows))
	}

	if len(osWindows[0].Tabs) != 2 {
		t.Fatalf("Expected 2 tabs, got %d", len(osWindows[0].Tabs))
	}

	// Check first tab (codex)
	codexTab := osWindows[0].Tabs[0]
	if codexTab.Title != "codex" {
		t.Errorf("Expected tab title 'codex', got %q", codexTab.Title)
	}
	if len(codexTab.Windows) != 1 {
		t.Fatalf("Expected 1 window in codex tab, got %d", len(codexTab.Windows))
	}

	codexWin := codexTab.Windows[0]
	if len(codexWin.ForegroundProcesses) != 3 {
		t.Fatalf("Expected 3 foreground processes, got %d", len(codexWin.ForegroundProcesses))
	}

	// Test extractAI on the codex window
	prefixes := []string{"codex", "claude", "gemini"}
	ai, ok := extractAI(codexWin, prefixes)
	if !ok {
		t.Error("extractAI should have found codex")
	}
	if ai != "codex" {
		t.Errorf("Expected AI 'codex', got %q", ai)
	}

	// Test extractAI on the lazyccg window (should not match)
	lazyccgWin := osWindows[0].Tabs[1].Windows[0]
	ai, ok = extractAI(lazyccgWin, prefixes)
	if ok {
		t.Error("extractAI should NOT have found AI in lazyccg window")
	}
}

func TestInferStatus(t *testing.T) {
	tests := []struct {
		name   string
		lines  []string
		want   string
	}{
		{
			name:   "empty lines",
			lines:  []string{},
			want:   "IDLE",
		},
		{
			name:   "waiting for input",
			lines:  []string{"Processing...", "Waiting for user input"},
			want:   "WAITING",
		},
		{
			name:   "waiting for approval",
			lines:  []string{"Changes ready", "Press enter to approve"},
			want:   "WAITING",
		},
		{
			name:   "done",
			lines:  []string{"Task completed successfully"},
			want:   "DONE",
		},
		{
			name:   "running",
			lines:  []string{"Executing command...", "Reading files..."},
			want:   "RUNNING",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := inferStatus(tt.lines)
			if got != tt.want {
				t.Errorf("inferStatus() = %q, want %q", got, tt.want)
			}
		})
	}
}
