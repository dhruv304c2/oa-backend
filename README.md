# Mystery Game Agent API

An interactive API service for playing mystery investigation games. Players can spawn AI-powered character agents from mystery stories, interrogate them, present evidence, and submit theories to solve the case.

## Features

- **Character Agents**: Spawn AI agents that embody story characters with unique personalities, knowledge, and evidence
- **Interactive Interrogation**: Question characters and receive in-character responses
- **Evidence System**: Present evidence to characters and observe their reactions
- **Theory Scoring**: Submit your theory and get scored on accuracy compared to the actual story

## Prerequisites

- Go 1.21 or higher
- MongoDB instance
- Gemini API key

## Installation

1. Clone the repository:
```bash
git clone <repository-url>
cd oa-agents
```

2. Install dependencies:
```bash
go mod download
```

3. Create a `.env` file in the root directory:
```env
# Database
MONGODB_URI=mongodb://localhost:27017/case-gen

# AI Service
GEMINI_API_KEY=your_gemini_api_key

# CORS Configuration
ALLOWED_ORIGINS=http://localhost:5173,http://localhost:3000
# Optional: Allow all origins (development only)
# CORS_ALLOW_ALL=true
```

Or set environment variables directly:
```bash
export MONGODB_URI="mongodb://localhost:27017/case-gen"
export GEMINI_API_KEY="your-gemini-api-key"
export ALLOWED_ORIGINS="http://localhost:5173,http://localhost:3000"
```

4. Run the server:
```bash
go run main.go
```

The server will start on `http://localhost:8080`

## API Endpoints

All endpoints support CORS for browser-based applications.

### 1. Get Story Feed
Get a list of all available mystery stories.

**Endpoint:** `GET /feed`

**Response:**
```json
{
  "stories": [
    {
      "id": "6997afd2b9d056d4b23f0743",
      "title": "The Whispering Pines Conspiracy",
      "description": "The serene setting of Havenwood has been marred by tragedy...",
      "cover_image_url": "https://story-gen-cdn.s3.eu-north-1.amazonaws.com/images/995db588_cover.png",
      "created_at": "2026-02-20T00:50:26.330Z",
      "updated_at": "2026-02-20T00:50:26.330Z"
    }
  ],
  "count": 1
}
```

### 2. Get Story Details
Get detailed information about a specific story.

**Endpoints:**
- `GET /story?id=STORY_ID` (Query parameter style)
- `GET /stories/STORY_ID` (RESTful style)

**Response:**
```json
{
  "id": "6997afd2b9d056d4b23f0743",
  "title": "The Whispering Pines Conspiracy",
  "news_article": {
    "title": "Renowned Conservationist Found Dead...",
    "content": "Full news article content..."
  },
  "cover_image_url": "https://story-gen-cdn.s3.eu-north-1.amazonaws.com/images/995db588_cover.png",
  "characters": [
    {
      "id": "char_1",
      "name": "Agnes Finch",
      "description": "A woman in her 60s, weathered appearance...",
      "image_url": "https://story-gen-cdn.s3.eu-north-1.amazonaws.com/images/695ba7d0_character_char_1.png",
      "holds_evidence": [
        {
          "id": "evid_7",
          "title": "Testimony Transcript: Agnes Finch",
          "description": "Detailed account of Aggie finding the body, hearing an argument, and seeing a dark sedan leave the area.",
          "visual_description": "A typed document formatted as a police interview transcript",
          "image_url": "https://story-gen-cdn.s3.eu-north-1.amazonaws.com/images/695ba7d0_evidence_evid_7.png"
        }
      ],
      "knows_location_ids": ["loc_1", "loc_8"]
    }
  ],
  "locations": [
    {
      "id": "loc_1",
      "name": "Whispering Pines Reserve",
      "description": "A majestic natural reserve...",
      "image_url": "https://story-gen-cdn.s3.eu-north-1.amazonaws.com/images/695ba7d0_location_loc_1.png",
      "character_ids_in_location": ["char_1", "char_2", "char_3"]
    }
  ],
  "created_at": "2026-02-20T00:50:26.330Z"
}
```

### 3. Spawn Agent
Create a new character agent from a story.

**Endpoint:** `POST /spawn`

**Request Body:**
```json
{
  "story_id": "699785171e1a1099d76570b3",
  "character_id": "char_1"
}
```

**Response:**
```json
{
  "agent_id": "agent-173413"
}
```

**Error Response:**
```json
{
  "error": "Story not found"
}
```

### 4. Send Message
Send a message to an agent and receive their response.

**Endpoint:** `POST /message`

**Request Body:**
```json
{
  "agent_id": "agent-173413",
  "message": "What do you know about the victim?",
  "presented_evidence": ["evid_2", "evid_6"],  // optional
  "location_id": "loc_1"  // optional
}
```

**Response:**
```json
{
  "reply": "The victim was a complex man...",
  "revealed_evidences": ["evid_21"],
  "revealed_locations": ["loc_3"]
}
```

**Notes:**
- `presented_evidence`: Optional array of evidence IDs to show to the character
- `location_id`: Optional location ID to set the context of where the conversation is happening
- `revealed_evidences`: Evidence IDs the character reveals in their response
- `revealed_locations`: Location IDs the character mentions in their response

### 5. Score Theory
Submit your theory about the case and get scored.

**Endpoint:** `POST /score`

**Request Body:**
```json
{
  "story_id": "699785171e1a1099d76570b3",
  "theory": "I believe the butler did it in the library with the candlestick..."
}
```

**Response:**
```json
{
  "score": 75,
  "reason": "Correctly identified the culprit but missed the motive..."
}
```

## Usage Example

### Complete Investigation Flow

1. **Browse Available Stories**
```bash
# Get all stories
curl http://localhost:8080/feed

# Get specific story details (both formats work)
curl "http://localhost:8080/story?id=6998345881f15a0dd57b210b"
curl http://localhost:8080/stories/6998345881f15a0dd57b210b
```

2. **Start an Investigation**
```bash
# Spawn a character agent
curl -X POST http://localhost:8080/spawn \
    -H "Content-Type: application/json" \
    -d '{
      "story_id": "699785171e1a1099d76570b3",
      "character_id": "char_secretary"
    }'
```

3. **Initial Interrogation**
```bash
# Ask about their relationship with the victim
curl -X POST http://localhost:8080/message \
    -H "Content-Type: application/json" \
    -d '{
      "agent_id": "agent-173413",
      "message": "How well did you know the victim?"
    }'
```

4. **Gather Evidence**
```bash
# Ask if they have anything helpful
curl -X POST http://localhost:8080/message \
    -H "Content-Type: application/json" \
    -d '{
      "agent_id": "agent-173413",
      "message": "Do you have anything that might help with the investigation?"
    }'
```

5. **Interrogate at a Specific Location**
```bash
# Question them at the crime scene for more contextual responses
curl -X POST http://localhost:8080/message \
    -H "Content-Type: application/json" \
    -d '{
      "agent_id": "agent-173413",
      "message": "Can you show me exactly where you found the body?",
      "location_id": "loc_1"
    }'
```

6. **Present Evidence with Location Context**
```bash
# Show them evidence at a specific location
curl -X POST http://localhost:8080/message \
    -H "Content-Type: application/json" \
    -d '{
      "agent_id": "agent-173413",
      "message": "Do you recognize this item?",
      "presented_evidence": ["evid_2"],
      "location_id": "loc_6"
    }'
```

7. **Submit Your Theory**
```bash
# Once you've gathered enough information
curl -X POST http://localhost:8080/score \
    -H "Content-Type: application/json" \
    -d '{
      "story_id": "699785171e1a1099d76570b3",
      "theory": "Based on my investigation, I conclude that the secretary killed the councilman because she discovered he was planning to fire her. She used the password fragment to access his computer and found the termination letter, leading to a confrontation where she grabbed the letter opener from his desk."
    }'
```

## MongoDB Schema

The API expects stories in MongoDB with the following structure:

**Database:** `case-gen`
**Collection:** `stories`

```json
{
  "_id": "ObjectId",
  "story": {
    "title": "The Case of the Missing Councilman",
    "full_story": "Complete narrative with solution...",
    "news_article": {
      "title": "Councilman Found Dead",
      "content": "Initial public information..."
    },
    "starting_location_ids": ["loc_1"],
    "characters": [
      {
        "id": "char_1",
        "name": "Martha Stevens",
        "appearance_description": "Middle-aged woman with grey hair...",
        "personality_profile": "Nervous, loyal, detail-oriented...",
        "knowledge_base": "What the character knows about events...",
        "holds_evidence": [
          {
            "id": "evid_2",
            "title": "Aris's Field Notebook",
            "description": "Weather-worn notebook filled with cryptic ecological observations",
            "visual_description": "A well-used, weather-worn notebook with a sturdy cover"
          }
        ],
        "knows_location_ids": ["loc_1", "loc_2"]
      }
    ],
    "locations": [
      {
        "id": "loc_1",
        "location_name": "Councilman's Office",
        "visual_description": "Luxurious office with mahogany desk",
        "character_ids_in_location": ["char_1", "char_2"]
      }
    ]
  },
  "created_at": "2024-01-01T00:00:00Z",
  "updated_at": "2024-01-01T00:00:00Z"
}
```

## Character Agent Behavior

### System Prompt Structure
When spawned, each agent receives:
- Character name and appearance
- Personality profile
- Knowledge base
- Evidence they possess (with IDs)
- Locations they know
- Full story context (for accurate responses)

### Response Format
Agents respond in JSON with:
- `reply`: Natural conversation response
- `revealed_evidences`: Evidence IDs revealed in this response
- `revealed_locations`: Location IDs mentioned in this response

### Behavioral Rules
- Characters stay in character based on personality profiles
- Can only reveal evidence they possess
- Can only mention locations they know
- Maintain conversation history throughout session
- React to presented evidence based on character knowledge

## Scoring System

The scoring algorithm evaluates theories on:

| Criteria | Points | Description |
|----------|--------|-------------|
| Culprit Identification | 40 | Correctly naming who committed the crime |
| Motive Understanding | 20 | Understanding why the crime was committed |
| Sequence of Events | 20 | Correct order and details of what happened |
| Key Evidence | 10 | Identifying crucial evidence |
| Character Relationships | 10 | Understanding interpersonal dynamics |

**Note:** Incorrectly identifying the culprit caps the maximum score at 60 points.

## Error Handling

| Status Code | Meaning | Common Causes |
|------------|---------|---------------|
| 400 | Bad Request | Invalid JSON, missing fields, invalid ObjectID |
| 404 | Not Found | Wrong endpoint, invalid story/character/agent ID |
| 405 | Method Not Allowed | Using GET instead of POST |
| 500 | Internal Server Error | Database connection, AI service issues |

## CORS Configuration

The API includes CORS support for browser-based applications. By default, it allows:
- Origins: `http://localhost:5173` (Vite), `http://localhost:3000` (React), and `*` for other origins
- Methods: `GET`, `POST`, `OPTIONS`
- Headers: `Content-Type`, `Authorization`

To customize CORS settings for production, edit `middleware/cors.go` and update the allowed origins.

## Development

### Project Structure
```
oa-agents/
├── main.go              # Server entry point and route registration
├── handlers/            # HTTP request handlers
│   ├── spawn.go        # Agent spawning logic
│   ├── message.go      # Message handling and evidence presentation
│   ├── score.go        # Theory scoring
│   ├── feed.go         # Story feed endpoints
│   └── story_restful.go # RESTful story endpoint
├── agent/              # Agent management
│   ├── agent.go        # Agent struct definition
│   └── registry.go     # Agent registry and spawning
├── models/             # Data models
│   └── story.go        # Story, Character, Evidence structures
├── middleware/         # HTTP middleware
│   └── cors.go         # CORS configuration
├── db/                 # Database
│   └── mongo.go        # MongoDB connection management
└── .env               # Environment variables (create this)
```

### Adding New Features

To add new character behaviors:
1. Modify the system prompt in `handlers/spawn.go`
2. Update the Agent struct in `agent/agent.go` if needed
3. Adjust message handling in `handlers/message.go`

## Tips for Players

1. **Start with open questions** to gauge character personalities
2. **Note evidence IDs** when characters reveal them
3. **Cross-reference information** between multiple characters
4. **Present evidence strategically** to get reactions
5. **Pay attention to what characters don't say** - it might be important

## License

[Your license here]

## Contributing

[Contribution guidelines if applicable]