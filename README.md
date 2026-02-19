# Agent API

A Go-based API for spawning and managing AI agents that can role-play as characters from a story.

## Environment Variables

Set the following environment variables:

```bash
export MONGODB_URI="mongodb://localhost:27017/case-gen"
export GEMINI_API_KEY="your-gemini-api-key"
```

## API Endpoints

### POST /spawn

Spawns a new agent as a character from a story.

**Request Body:**
```json
{
  "story_id": "699780ce5a780332d8a3fbab",
  "character_id": "char_5"
}
```

**Response:**
```json
{
  "agent_id": "agent-123456"
}
```

### POST /message

Send a message to an existing agent.

**Request Body:**
```json
{
  "agent_id": "agent-123456",
  "message": "What did you argue with Elias about?"
}
```

**Response:**
```json
{
  "reply": "Character's response based on their personality and knowledge..."
}
```

## MongoDB Schema

The API expects stories in MongoDB with the following structure:
- Database: `case-gen`
- Collection: `stories`
- Evidence objects include an `id` field (e.g., "evid_21", "evid_10") for unique identification

## How It Works

1. When spawning an agent, the system:
   - Fetches the story from MongoDB using the story_id
   - Finds the character within the story using character_id
   - Constructs a system prompt that includes:
     - Character's name, appearance, and personality
     - Character's knowledge base
     - Any evidence the character holds (with evidence IDs)
     - Locations the character knows
   - Includes the full story as context in a separate content block
   - Creates an agent that will respond as this character

2. The agent will:
   - Stay in character based on their personality profile
   - Only share information from their knowledge base
   - React according to their personality traits
   - Maintain consistency with their role in the story