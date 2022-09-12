// compileFunctions compiles all functions in compilequeue.
// It fans out nBackendWorkers to do the work
// and waits for them to complete.
func compileFunctions() {
	if len(compilequeue) == 0 {
		return
	}

	if race.Enabled {
		// Randomize compilation order to try to shake out races.
		tmp := make([]*ir.Func, len(compilequeue))
		perm := rand.Perm(len(compilequeue))
		for i, v := range perm {
			tmp[v] = compilequeue[i]
		}
		copy(compilequeue, tmp)
	} else {
		// Compile the longest functions first,
		// since they're most likely to be the slowest.
		// This helps avoid stragglers.
		sort.Slice(compilequeue, func(i, j int) bool {
			return len(compilequeue[i].Body) > len(compilequeue[j].Body)
		})
	}

	// By default, we perform work right away on the current goroutine
	// as the solo worker.
	queue := func(work func(int)) {
		work(0)
	}

	if nWorkers := base.Flag.LowerC; nWorkers > 1 {
		// For concurrent builds, we create a goroutine per task, but
		// require them to hold a unique worker ID while performing work
		// to limit parallelism.
		workerIDs := make(chan int, nWorkers)
		for i := 0; i < nWorkers; i++ {
			workerIDs <- i
		}

		queue = func(work func(int)) {
			go func() {
				worker := <-workerIDs
				work(worker)
				workerIDs <- worker
			}()
		}
	}

	var wg sync.WaitGroup
	var compile func([]*ir.Func)
	compile = func(fns []*ir.Func) {
		wg.Add(len(fns))
		for _, fn := range fns {
			fn := fn
			queue(func(worker int) {
				ssagen.Compile(fn, worker)
				compile(fn.Closures)
				wg.Done()
			})
		}
	}

	types.CalcSizeDisabled = true // not safe to calculate sizes concurrently
	base.Ctxt.InParallel = true

	compile(compilequeue)
	compilequeue = nil
	wg.Wait()

	base.Ctxt.InParallel = false
	types.CalcSizeDisabled = false
}

