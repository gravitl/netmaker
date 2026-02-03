# Scale Testing Guide for GetNodeStatus Optimization

This document explains how to run scale tests to verify the performance improvements in the optimized `GetNodeStatus` function.

## Overview

The scale tests verify that the optimized `GetNodeStatus` function performs well with:
- Varying numbers of peers (10, 50, 100, 200)
- Varying numbers of ACL policies (5, 20, 50, 100)
- With and without default policy enabled

## Running Tests

### Run All Scale Tests

```bash
# Run all scale tests (may take several minutes)
go test -v ./pro/logic -run TestGetNodeStatusScale

# Skip scale tests in short mode
go test -v ./pro/logic -short
```

### Run Specific Test Cases

```bash
# Small scale test
go test -v ./pro/logic -run TestGetNodeStatusScale/Small

# Medium scale test
go test -v ./pro/logic -run TestGetNodeStatusScale/Medium

# Large scale test
go test -v ./pro/logic -run TestGetNodeStatusScale/Large

# Very large scale test
go test -v ./pro/logic -run TestGetNodeStatusScale/Very
```

## Running Benchmarks

### Run All Benchmarks

```bash
# Run all benchmarks
go test -bench=. ./pro/logic -benchmem

# Run with more iterations for better accuracy
go test -bench=. ./pro/logic -benchmem -benchtime=10s
```

### Run Specific Benchmarks

```bash
# Benchmark with 10 peers and 5 policies
go test -bench=BenchmarkGetNodeStatus/10peers_5policies ./pro/logic -benchmem

# Benchmark with 100 peers and 50 policies
go test -bench=BenchmarkGetNodeStatus/100peers_50policies ./pro/logic -benchmem

# Benchmark ACL filtering optimization
go test -bench=BenchmarkACLFiltering ./pro/logic -benchmem
```

### Compare Before/After Performance

To compare performance before and after optimization:

1. **Before optimization** (if you have the old code):
   ```bash
   git checkout <old-commit>
   go test -bench=BenchmarkGetNodeStatus ./pro/logic -benchmem > before.txt
   ```

2. **After optimization**:
   ```bash
   git checkout <new-commit>
   go test -bench=BenchmarkGetNodeStatus ./pro/logic -benchmem > after.txt
   ```

3. **Compare results**:
   ```bash
   diff before.txt after.txt
   ```

## Expected Performance Improvements

Based on the optimizations:

1. **Policy Pre-fetching**: Reduces database calls from N (number of peers) to 1
2. **Node Tag Pre-computation**: Reduces tag computation from N to 1
3. **Policy Filtering**: Reduces policy iterations per peer check

### Expected Results

- **Small scale (10 peers, 5 policies)**: < 100ms
- **Medium scale (50 peers, 20 policies)**: < 500ms
- **Large scale (100 peers, 50 policies)**: < 2s
- **Very large scale (200 peers, 100 policies)**: < 5s

With default policy enabled, performance should be even better as ACL checks are skipped.

## Test Scenarios

The tests create realistic scenarios:

1. **Network Setup**: Creates a test network with proper addressing
2. **Node Creation**: Creates a test node and multiple peer nodes
3. **ACL Policies**: Creates various ACL policies:
   - Policies matching test node by ID
   - Policies with wildcards (matching all nodes)
   - Policies matching specific peers
4. **Metrics**: Creates connectivity metrics for all peers
5. **Status Check**: Runs `GetNodeStatus` and measures performance

## Interpreting Results

### Test Output Example

```
=== RUN   TestGetNodeStatusScale/Small_scale:_10_peers,_5_policies
    status_test.go:XX: GetNodeStatus completed in 45ms for 10 peers and 5 policies (defaultPolicy=false)
    status_test.go:XX: Average time per peer: 4.5ms
--- PASS: TestGetNodeStatusScale (0.05s)
```

### Benchmark Output Example

```
BenchmarkGetNodeStatus/100peers_50policies-8         100    12500000 ns/op    5000000 B/op    50000 allocs/op
```

- **ns/op**: Nanoseconds per operation
- **B/op**: Bytes allocated per operation
- **allocs/op**: Number of allocations per operation

## Troubleshooting

### Tests Fail with Database Errors

Ensure you have a clean test database:
```bash
# The tests initialize their own test database, but if you see errors:
rm -rf data/test.db
```

### Tests Take Too Long

If tests are taking too long, you can:
1. Reduce the number of peers/policies in test cases
2. Run with `-short` flag to skip large-scale tests
3. Run specific test cases instead of all

### Out of Memory

If you encounter memory issues:
1. Reduce the scale of test scenarios
2. Run tests one at a time
3. Ensure you have sufficient system memory

## Continuous Integration

For CI/CD pipelines, consider:

```yaml
# Example GitHub Actions workflow
- name: Run Scale Tests
  run: |
    go test -v ./pro/logic -run TestGetNodeStatusScale -timeout 10m
    
- name: Run Benchmarks
  run: |
    go test -bench=BenchmarkGetNodeStatus ./pro/logic -benchmem -benchtime=5s
```

## Next Steps

1. Run the scale tests to establish baseline performance
2. Monitor performance over time as the codebase evolves
3. Add more test scenarios as needed (e.g., static nodes, ingress gateways)
4. Consider adding performance regression tests to CI/CD

