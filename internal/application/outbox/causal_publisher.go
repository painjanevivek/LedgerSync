package outbox

import "context"

type PublicationClock interface {
	PreparePublication(context.Context, string, Event) (Event, error)
}
type CausalPublisher struct {
	publisher Publisher
	clock     PublicationClock
	owner     string
}

func NewCausalPublisher(publisher Publisher, clock PublicationClock, owner string) *CausalPublisher {
	return &CausalPublisher{publisher: publisher, clock: clock, owner: owner}
}
func (p *CausalPublisher) Publish(ctx context.Context, event Event) error {
	prepared, err := p.clock.PreparePublication(ctx, p.owner, event)
	if err != nil {
		return err
	}
	return p.publisher.Publish(ctx, prepared)
}
