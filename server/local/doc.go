// Package local contains server implementations that are backed by
// stores. It is called "local" because it accesses data directly from
// the underlying stores instead of merely wrapping, routing, or
// proxying gRPC method calls.
//
// Local methods should contain the "business logic" for each
// method. They should be independent of where or how the data is
// stored or retrieved. The storage logic belongs in the stores (e.g.,
// server/internal/store/fs, server/internal/store/pgsql, ext/github,
// etc.).
package local
