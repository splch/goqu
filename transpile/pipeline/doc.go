// Package pipeline provides pre-built transpilation pipelines with four
// optimization levels, following the industry-standard quantum compilation
// flow: init → pre-route optimize → route → translate → optimize → validate.
//
//   - [LevelNone]:     decompose + route + translate + validate (no optimization)
//   - [LevelBasic]:    + pre-routing cancel/merge, post-routing iterative cancel/merge
//   - [LevelFull]:     + 2Q block consolidation, commutation, parallelization (iterative)
//   - [LevelParallel]: runs multiple strategies concurrently, picks lowest cost
//
// [DefaultPipeline] returns a [transpile.Pass] for a given level.
// [Run] adds context-based observability hooks around each pass.
package pipeline
