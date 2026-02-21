# Scoring Enhancement: Evidence Discovery Tracking

## Overview

The scoring endpoint has been enhanced to consider what evidence the player has actually discovered during their investigation. This provides more nuanced scoring that rewards thorough detective work and proper evidence interpretation.

## API Changes

### Updated Request Structure

```json
POST /score
{
  "story_id": "507f1f77bcf86cd799439011",
  "theory": "I believe the butler committed the murder...",
  "discovered_evidence": ["evidence_001", "evidence_002", "evidence_003"]  // Optional
}
```

### Fields

- `story_id` (required): The MongoDB ObjectID of the story
- `theory` (required): The player's theory about what happened
- `discovered_evidence` (optional): Array of evidence IDs the player has discovered

### Response

```json
{
  "score": 75,
  "reason": "Correctly identified the culprit and motive. Good use of fingerprint evidence (evidence_001) and witness testimony (evidence_002). However, missed the crucial timeline evidence (evidence_004) that would have strengthened the sequence of events."
}
```

## Scoring Breakdown

The enhanced scoring system evaluates theories based on:

1. **Correct identification of the culprit** (30 points)
   - Did they identify the right person?

2. **Understanding of motive** (20 points)
   - Do they understand why the crime was committed?

3. **Correct sequence of events** (20 points)
   - Can they accurately describe how events unfolded?

4. **Effective use of discovered evidence** (20 points)
   - Did they find the right evidence?
   - Did they correctly interpret the evidence?
   - Is their theory supported by the evidence they found?

5. **Understanding of relationships** (10 points)
   - Do they understand character relationships and dynamics?

## Evidence Context

The scoring AI now receives context about what evidence the player has discovered:

- If no evidence is provided, scoring works as before (backward compatible)
- If evidence IDs are provided, the AI considers:
  - Whether the player found crucial evidence
  - How well they interpreted the evidence they found
  - What important evidence they might have missed

## Benefits

1. **More Accurate Scoring**: Theories are evaluated based on available evidence
2. **Better Feedback**: Players learn what evidence they missed
3. **Rewards Exploration**: Thorough investigators score higher
4. **Realistic Detective Work**: Theories need evidence support

## Example Usage

```bash
# Basic scoring without evidence
curl -X POST http://localhost:8081/score \
  -H "Content-Type: application/json" \
  -d '{
    "story_id": "507f1f77bcf86cd799439011",
    "theory": "The butler did it with the candlestick in the library."
  }'

# Enhanced scoring with discovered evidence
curl -X POST http://localhost:8081/score \
  -H "Content-Type: application/json" \
  -d '{
    "story_id": "507f1f77bcf86cd799439011",
    "theory": "The butler did it with the candlestick in the library.",
    "discovered_evidence": ["fingerprints_001", "witness_testimony_002", "security_footage_003"]
  }'
```

## Implementation Details

- Uses existing `fetchEvidenceDetails` function from message handler
- Maintains backward compatibility when no evidence is provided
- Formats evidence clearly for AI evaluation
- Gracefully handles invalid evidence IDs