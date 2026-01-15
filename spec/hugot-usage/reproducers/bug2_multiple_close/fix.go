// Bug 2 Fix: Ensure Close() only runs once
//
// This shows the minimal change to prevent multiple/concurrent Close() crashes.

package bug2

/*
CURRENT CODE (hugot.go):
========================

func (p *PooledHugotEmbedder) Close() error {
	if p.session != nil && !p.sessionShared {
		p.logger.Info("Destroying Hugot session (owned by this pooled embedder)")
		return p.session.Destroy()  // ← BUG: Can be called multiple times!
	} else if p.sessionShared {
		p.logger.Debug("Skipping session destruction (shared session)")
	}
	return nil
}


FIXED CODE:
===========

// Add to struct:
type PooledHugotEmbedder struct {
	// ... existing fields ...
	closeOnce sync.Once
	closeErr  error
}

func (p *PooledHugotEmbedder) Close() error {
	p.closeOnce.Do(func() {
		if p.session != nil && !p.sessionShared {
			p.logger.Info("Destroying Hugot session (owned by this pooled embedder)")
			p.closeErr = p.session.Destroy()
		} else if p.sessionShared {
			p.logger.Debug("Skipping session destruction (shared session)")
		}
	})
	return p.closeErr
}


COMBINED FIX (with Bug 1):
==========================

// The full fix addresses both bugs:
// - Bug 1: Close() during Embed() → Use WaitGroup + atomic.Bool
// - Bug 2: Multiple Close() calls → Use sync.Once

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

	// Synchronization (fixes both bugs)
	closed    atomic.Bool
	wg        sync.WaitGroup
	closeOnce sync.Once
	closeErr  error
}

func (p *PooledHugotEmbedder) Embed(ctx context.Context, contents [][]ai.ContentPart) ([][]float32, error) {
	if p.closed.Load() {
		return nil, errors.New("embedder is closed")
	}

	p.wg.Add(1)
	defer p.wg.Done()

	if p.closed.Load() {
		return nil, errors.New("embedder is closed")
	}

	// ... rest unchanged ...
}

func (p *PooledHugotEmbedder) Close() error {
	p.closeOnce.Do(func() {
		p.closed.Store(true)
		p.wg.Wait()

		if p.session != nil && !p.sessionShared {
			p.logger.Info("Destroying Hugot session (owned by this pooled embedder)")
			p.closeErr = p.session.Destroy()
		} else if p.sessionShared {
			p.logger.Debug("Skipping session destruction (shared session)")
		}
	})
	return p.closeErr
}
*/
