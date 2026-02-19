#!/bin/bash

# Test script for the Agent API

# Example 1: Spawn an agent as Marcus Thorne (no evidence)
echo "Example 1: Spawning agent as Marcus Thorne..."
RESPONSE=$(curl -s -X POST http://localhost:8080/spawn \
  -H "Content-Type: application/json" \
  -d '{
    "story_id": "699780ce5a780332d8a3fbab",
    "character_id": "char_5"
  }')

echo "Response: $RESPONSE"

# Extract agent_id from response
AGENT_ID=$(echo $RESPONSE | grep -o '"agent_id":"[^"]*' | cut -d'"' -f4)

if [ -n "$AGENT_ID" ]; then
  echo -e "\nAgent spawned with ID: $AGENT_ID"

  # Send initial message
  echo -e "\nSending initial message..."
  curl -X POST http://localhost:8080/message \
    -H "Content-Type: application/json" \
    -d "{
      \"agent_id\": \"$AGENT_ID\",
      \"message\": \"Hello Marcus, I heard you had some disagreements with Elias. Can you tell me about that?\"
    }"

  echo -e "\n\n---\n"

  # Send follow-up message
  echo "Sending follow-up message..."
  curl -X POST http://localhost:8080/message \
    -H "Content-Type: application/json" \
    -d "{
      \"agent_id\": \"$AGENT_ID\",
      \"message\": \"What was your relationship with Elias like growing up?\"
    }"
  echo ""
else
  echo "Failed to spawn agent"
fi

echo -e "\n\n=== Example 2 ===\n"

# Example 2: Spawn an agent as Marcus Kaine (has evidence)
echo "Example 2: Spawning agent as Marcus Kaine who holds evidence..."
RESPONSE2=$(curl -s -X POST http://localhost:8080/spawn \
  -H "Content-Type: application/json" \
  -d '{
    "story_id": "699785171e1a1099d76570b3",
    "character_id": "char_5"
  }')

echo "Response: $RESPONSE2"

# Extract agent_id from second response
AGENT_ID2=$(echo $RESPONSE2 | grep -o '"agent_id":"[^"]*' | cut -d'"' -f4)

if [ -n "$AGENT_ID2" ]; then
  echo -e "\nSecond agent spawned with ID: $AGENT_ID2"

  # Send initial message
  echo -e "\nSending message to second agent..."
  curl -X POST http://localhost:8080/message \
    -H "Content-Type: application/json" \
    -d "{
      \"agent_id\": \"$AGENT_ID2\",
      \"message\": \"Marcus, what evidence do you have about the Obsidian Heart?\"
    }"
  echo ""
else
  echo "Failed to spawn second agent"
fi