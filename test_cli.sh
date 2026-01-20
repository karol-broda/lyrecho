#!/bin/bash
echo "=== Testing lyrecho CLI ==="
echo

echo "1. Version:"
./lyrecho --version
echo

echo "2. Cache stats:"
./lyrecho cache stats
echo

echo "3. Player list:"
./lyrecho player list
echo

echo "4. Cache list (first 5):"
./lyrecho cache list | head -7
echo

echo "5. Fuzzy matching test (missing !):"
./lyrecho cache show "Chappell Roan" "HOT TO GO" 2>&1 || true
echo

echo "6. Exact match:"
./lyrecho cache show 'Chappell Roan' 'HOT TO GO!' | head -5
echo

echo "=== All tests complete! ==="
