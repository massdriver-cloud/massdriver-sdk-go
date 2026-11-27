package streaming

// Stream is one open subscription. Receive from C until it closes, then
// call Err to learn why it closed:
//
//	stream, err := c.Instances.StreamEvents(ctx, "ecomm-prod-database")
//	if err != nil {
//	    return err
//	}
//	for ev := range stream.C {
//	    // handle ev
//	}
//	if err := stream.Err(); err != nil {
//	    return err // the connection failed
//	}
//
// Err is nil when the caller cancelled the context or the server completed
// the subscription, and the connection's error otherwise.
type Stream[T any] struct {
	// C yields each item. It closes when the context is cancelled, the
	// server completes the subscription, or the connection fails.
	C <-chan T

	err func() error
}

// New returns a Stream that yields from c. err reports why c closed and is
// called only through [Stream.Err]. A nil err always reports nil, which
// suits fakes in tests.
func New[T any](c <-chan T, err func() error) *Stream[T] {
	return &Stream[T]{C: c, err: err}
}

// Err reports why C closed. Call it after C has closed; before that its
// result is not final.
func (s *Stream[T]) Err() error {
	if s.err == nil {
		return nil
	}
	return s.err()
}
