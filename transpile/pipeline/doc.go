// Package pipeline provides pre-built transpilation pipelines with four
// optimization levels.
//
//   - [LevelNone]:     decompose to target basis only
//   - [LevelBasic]:    + adjacent gate cancellation and rotation merging
//   - [LevelFull]:     + commutation and parallelization
//   - [LevelParallel]: runs multiple strategies concurrently, picks lowest cost
//
// [DefaultPipeline] returns a [transpile.Pass] for a given level.
// [Run] adds context-based observability hooks around each pass.
package pipeline
