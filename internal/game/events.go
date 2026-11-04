package game

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/SatoriAI/MT2-Go/internal/event"
	"github.com/SatoriAI/MT2-Go/internal/protocol"
)

// Events is what the game needs of the stored events.
type Events interface {
	Running(ctx context.Context, now time.Time) (event.Event, bool, error)
}

// WatchEvents looks for the running event every so often until ctx ends, and switches it
// on and off. Events are created from the command line, another process, so the server
// reads them rather than being told. A failed read keeps what runs until the next look,
// and a look takes at most until the next, so a database that stops answering is noticed.
func (s *Service) WatchEvents(ctx context.Context, events Events, every time.Duration, log *slog.Logger) error {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		e, running, err := lookForEvent(ctx, events, every)
		switch {
		case ctx.Err() != nil:
			return nil
		case err != nil:
			log.Warn("events not checked", "err", err)
		default:
			s.presence.switchEvent(e, running)
		}
		select {
		case <-ctx.Done():
			return nil
		case <-t.C:
		}
	}
}

func lookForEvent(ctx context.Context, events Events, within time.Duration) (event.Event, bool, error) {
	ctx, cancel := context.WithTimeout(ctx, within)
	defer cancel()
	return events.Running(ctx, time.Now())
}

// switchEvent makes e the running event, or none, and tells every player of the one that
// ended and the one that began.
func (w *presence) switchEvent(e event.Event, running bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if (w.event == nil && !running) || (w.event != nil && running && w.event.ID == e.ID) {
		return
	}
	var notices [][]byte
	if w.event != nil {
		notices = append(notices, notice(fmt.Sprintf("Wydarzenie „%s” dobiegło końca.", w.event.Name)))
		w.event = nil
	}
	if running {
		w.event = &e
		notices = append(notices, eventNotice(e))
	}
	for _, p := range w.byName {
		p.send(notices...)
	}
}

// eventNotice tells a player an event is running, until when, and what it gives.
func eventNotice(e event.Event) []byte {
	return notice(fmt.Sprintf("Wydarzenie „%s” trwa do %s: doświadczenie ×%s.", e.Name,
		e.End.In(event.Zone).Format("02.01 15:04"), event.Multiplier(e.ExpPercent)))
}

// notice is a line from the server itself: from vid 0, which the client shows at the top
// of the screen and in chat.
func notice(text string) []byte {
	return protocol.EncodeChat(protocol.ChatNotice, 0, 0, textToClient(text))
}
