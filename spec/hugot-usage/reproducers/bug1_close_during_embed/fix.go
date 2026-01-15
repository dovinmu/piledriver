// Bug 1 Fix: Add synchronization to Close()
//
// This shows the minimal changes needed to fix the Close() race condition.

package bug1

/*
CURRENT CODE (hugot.go):
========================

type PooledHugotEmbedder struct {
	session       *khugot.Session
	pipelines     []*pipelines.FeatureExtractionPipeline
	sem           *semaphore.Weighted
	nextPipeline  atomic.Uint64
	logger        *zap.Logger
	sessionShared bool
	poolSize      int
	caps          embeddings.EmbedderCapabilities
	batchSize     int
}

func (p *PooledHugotEmbedder) Close() error {
	if p.session != nil && !p.sessionShared {
		p.logger.Info("Destroying Hugot session (owned by this pooled embedder)")
		return p.session.Destroy()  // ← BUG: No synchronization!
	}
	return nil
}


FIXED CODE:
===========

type PooledHugotEmbedder struct {
	session       *khugot.Session
	pipelines     []*pipelines.FeatureExtractionPipeline
	sem           *semaphore.Weighted
	nextPipeline  atomic.Uint64
	logger        *zap.Logger
	sessionShared bool
	poolSize      int
	caps          embeddings.EmbedderCapabilities
	batchSize     int

	// NEW: Synchronization for Close()
	closed    atomic.Bool    // Prevents new Embed() calls after Close()
	wg        sync.WaitGroup // Waits for in-flight Embed() calls
	closeOnce sync.Once      // Ensures Close() only runs once
}

func (p *PooledHugotEmbedder) Embed(ctx context.Context, contents [][]ai.ContentPart) ([][]float32, error) {
	// NEW: Check if closed
	if p.closed.Load() {
		return nil, errors.New("embedder is closed")
	}

	// NEW: Register in-flight operation
	p.wg.Add(1)
	defer p.wg.Done()

	// NEW: Double-check after registration (handles race with Close)
	if p.closed.Load() {
		return nil, errors.New("embedder is closed")
	}

	// ... rest of existing Embed() code unchanged ...
}

func (p *PooledHugotEmbedder) Close() error {
	var closeErr error

	p.closeOnce.Do(func() {
		// Set closed flag first - prevents new Embed() calls
		p.closed.Store(true)

		// Wait for all in-flight Embed() calls to complete
		p.wg.Wait()

		// Now safe to destroy session
		if p.session != nil && !p.sessionShared {
			p.logger.Info("Destroying Hugot session (owned by this pooled embedder)")
			closeErr = p.session.Destroy()
		} else if p.sessionShared {
			p.logger.Debug("Skipping session destruction (shared session)")
		}
	})

	return closeErr
}
*/
