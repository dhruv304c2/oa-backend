#!/bin/bash

# Test script for the enhanced scoring endpoint with evidence tracking

echo "Testing Enhanced Scoring Endpoint"
echo "================================="

# Test 1: Without discovered evidence (backward compatibility)
echo -e "\n1. Testing without evidence (backward compatibility):"
curl -X POST http://localhost:8081/score \
  -H "Content-Type: application/json" \
  -d '{
    "story_id": "YOUR_STORY_ID_HERE",
    "theory": "I believe the butler did it because of the fingerprints on the knife."
  }' | jq .

# Test 2: With discovered evidence
echo -e "\n2. Testing with discovered evidence:"
curl -X POST http://localhost:8081/score \
  -H "Content-Type: application/json" \
  -d '{
    "story_id": "YOUR_STORY_ID_HERE",
    "theory": "I believe the butler did it because of the fingerprints on the knife.",
    "discovered_evidence": ["evidence_001", "evidence_002"]
  }' | jq .

# Test 3: With empty evidence array
echo -e "\n3. Testing with empty evidence array:"
curl -X POST http://localhost:8081/score \
  -H "Content-Type: application/json" \
  -d '{
    "story_id": "YOUR_STORY_ID_HERE",
    "theory": "I believe the butler did it because of the fingerprints on the knife.",
    "discovered_evidence": []
  }' | jq .