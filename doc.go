// Package forma builds deterministic project scaffolds.
//
// Components describe output without accessing a destination. A Builder
// validates components and produces an immutable Plan. Inspect reports
// destination changes, and Apply performs them with explicit conflict policies.
package forma
