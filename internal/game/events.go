package game

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/SatoriAI/MT2-Go/internal/event"
)

// Events is what the game needs of the stored events.
type Events interface {
	Running(ctx context.Context, now time.Time) (event.Event, bool, error)
}

// WatchEvents looks for the running event every so often until ctx ends, and switches it
// on and off. Events are created from the command line, another process, so the server
// reads them rather than being told. A failed read keeps what runs until the next look,
// and a look takes at most until the next, so a database that stops answering is noticed.
func (svc *Service) WatchEvents(ctx context.Context, events Events, every time.Duration, log *slog.Logger) error {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		e, running, err := lookForEvent(ctx, events, every, svc.now())
		switch {
		case ctx.Err() != nil:
			return nil
		case err != nil:
			log.Warn("events not checked", "err", err)
		default:
			svc.record.switchEvent(e, running)
		}
		select {
		case <-ctx.Done():
			return nil
		case <-t.C:
		}
	}
}

func lookForEvent(ctx context.Context, events Events, within time.Duration, now time.Time) (event.Event, bool, error) {
	ctx, cancel := context.WithTimeout(ctx, within)
	defer cancel()
	return events.Running(ctx, now)
}

// switchEvent makes e the running event, or none, and tells every player of the one that
// ended and the one that began.
func (w *record) switchEvent(e event.Event, running bool) {
	l, unlock := w.lock()
	defer unlock()
	if (l.event == nil && !running) || (l.event != nil && running && l.event.ID == e.ID) {
		return
	}
	var notices [][]byte
	if l.event != nil {
		notices = append(notices, notice(fmt.Sprintf("Wydarzenie „%s” dobiegło końca.", l.event.Name)))
		l.event = nil
	}
	if running {
		l.event = &e
		notices = append(notices, eventNotice(e))
	}
	for _, p := range l.byName {
		p.send(notices...)
	}
}

// eventNotice tells a player an event is running, until when, and what it gives: only
// what it multiplies is named.
func eventNotice(e event.Event) []byte {
	var gives []string
	if e.ExpPercent > event.MinPercent {
		gives = append(gives, "doświadczenie ×"+event.Multiplier(e.ExpPercent))
	}
	if e.YangPercent > event.MinPercent {
		gives = append(gives, "yang ×"+event.Multiplier(e.YangPercent))
	}
	return notice(fmt.Sprintf("Wydarzenie „%s” trwa do %s: %s.", e.Name, e.End.In(event.Zone).Format("02.01 15:04"),
		strings.Join(gives, ", ")))
}
