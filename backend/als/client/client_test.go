package client

import (
	"testing"
	"time"
)

// clearClients empties the global Clients map. Call this at the start of each
// subtest; tests run sequentially so this is safe.
func clearClients() {
	clientsMu.Lock()
	Clients = make(map[string]*ClientSession)
	clientsMu.Unlock()
}

func TestRemoveExpiredClients(t *testing.T) {
	clearClients()

	tests := []struct {
		name         string
		seed         map[string]time.Time
		wantRemoved  int
		wantRemainID []string
	}{
		{
			name:        "no clients",
			seed:        map[string]time.Time{},
			wantRemoved: 0,
		},
		{
			name: "all fresh",
			seed: map[string]time.Time{
				"a": time.Now(),
				"b": time.Now(),
			},
			wantRemoved:  0,
			wantRemainID: []string{"a", "b"},
		},
		{
			name: "all expired",
			seed: map[string]time.Time{
				"a": time.Now().Add(-25 * time.Hour),
				"b": time.Now().Add(-48 * time.Hour),
			},
			wantRemoved: 2,
		},
		{
			name: "mixed",
			seed: map[string]time.Time{
				"fresh": time.Now(),
				"old1":  time.Now().Add(-25 * time.Hour),
				"old2":  time.Now().Add(-72 * time.Hour),
			},
			wantRemoved:  2,
			wantRemainID: []string{"fresh"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearClients()

			for id, createdAt := range tt.seed {
				Clients[id] = &ClientSession{
					Channel:   make(chan *Message, 1),
					CreatedAt: createdAt,
				}
			}

			got := RemoveExpiredClients()
			if got != tt.wantRemoved {
				t.Errorf("RemoveExpiredClients() = %d; want %d", got, tt.wantRemoved)
			}

			if len(Clients) != len(tt.wantRemainID) {
				t.Errorf("remaining clients = %d; want %d", len(Clients), len(tt.wantRemainID))
			}
			for _, want := range tt.wantRemainID {
				if _, ok := Clients[want]; !ok {
					t.Errorf("expected client %q to remain", want)
				}
			}
		})
	}
}

func TestRemoveExpiredClientsDeletesExpiredSession(t *testing.T) {
	// RemoveExpiredClients no longer forces cancellation of in-flight
	// contexts -- the caller that called GetContext owns the cancel.
	// The session is simply removed from the map. The "InvokesCancel"
	// suffix in the original test name is misleading; this test pins
	// the deletion contract only.
	clearClients()

	Clients["expired"] = &ClientSession{
		Channel:   make(chan *Message, 1),
		CreatedAt: time.Now().Add(-25 * time.Hour),
	}

	if removed := RemoveExpiredClients(); removed != 1 {
		t.Errorf("removed = %d; want 1", removed)
	}
	if _, ok := Clients["expired"]; ok {
		t.Error("expired session still in Clients after RemoveExpiredClients")
	}
}

func TestBroadCastMessageDeliversToAll(t *testing.T) {
	clearClients()

	c1 := &ClientSession{Channel: make(chan *Message, 4), CreatedAt: time.Now()}
	c2 := &ClientSession{Channel: make(chan *Message, 4), CreatedAt: time.Now()}
	Clients["c1"] = c1
	Clients["c2"] = c2

	BroadCastMessage("evt", "hello")

	for _, c := range []*ClientSession{c1, c2} {
		select {
		case msg := <-c.Channel:
			if msg.Name != "evt" || msg.Content != "hello" {
				t.Errorf("got %+v; want {evt hello}", msg)
			}
		case <-time.After(time.Second):
			t.Fatal("did not receive broadcast in time")
		}
	}
}

func TestBroadCastMessageDropsWhenBufferFull(t *testing.T) {
	clearClients()

	c := &ClientSession{Channel: make(chan *Message, 1), CreatedAt: time.Now()}
	Clients["c"] = c
	c.Channel <- &Message{Name: "filler", Content: "x"}

	BroadCastMessage("evt", "hello")

	select {
	case msg := <-c.Channel:
		if msg.Name != "filler" {
			t.Errorf("got unexpected message: %+v", msg)
		}
	default:
		t.Fatal("expected filler message in buffer")
	}
}

func TestGetClientExpiresOnAge(t *testing.T) {
	clearClients()

	Clients["stale"] = &ClientSession{
		Channel:   make(chan *Message, 1),
		CreatedAt: time.Now().Add(-25 * time.Hour),
	}

	if _, ok := GetClient("stale"); ok {
		t.Error("expected stale client to be reported as missing")
	}
}

func TestRemoveAllClients(t *testing.T) {
	clearClients()

	for _, id := range []string{"a", "b", "c"} {
		Clients[id] = &ClientSession{
			Channel:   make(chan *Message, 1),
			CreatedAt: time.Now(),
		}
	}
	if len(Clients) != 3 {
		t.Fatalf("setup: len(Clients) = %d; want 3", len(Clients))
	}

	RemoveAllClients()
	if len(Clients) != 0 {
		t.Errorf("after RemoveAllClients: len(Clients) = %d; want 0", len(Clients))
	}
}

func TestRemoveAllClientsOnEmptyMap(t *testing.T) {
	clearClients()

	// Calling on an empty (or nil) map must not panic and must
	// leave the map empty and usable.
	RemoveAllClients()
	if len(Clients) != 0 {
		t.Errorf("len(Clients) = %d; want 0", len(Clients))
	}

	// Re-add a client and verify the map is still functional.
	Clients["after-empty"] = &ClientSession{
		Channel:   make(chan *Message, 1),
		CreatedAt: time.Now(),
	}
	if _, ok := Clients["after-empty"]; !ok {
		t.Error("map not usable after RemoveAllClients on empty")
	}
}
