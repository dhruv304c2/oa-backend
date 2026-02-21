# Location Awareness Test Scenarios

## Test Scenario 1: Promise Consistency

### Setup
- Character: Any character with medical-related evidence or knowledge
- Current Location: Office or other non-medical location
- Evidence to discuss: Medical report or health-related information

### Test Steps
1. **Initial Promise**
   - User: "Tell me about the medical report"
   - Expected: Character promises to discuss at infirmary/medical facility

2. **Follow-up at Same Location**
   - User: "What about that medical report you mentioned?"
   - Expected: Character maintains promise, suggests going to infirmary
   - Phrases to look for:
     - "As I mentioned, I'd prefer to discuss that at the infirmary"
     - "Let's head to the infirmary first"
     - "I really think we should wait until we're at the infirmary"

3. **Persistence Test**
   - User: "Just tell me now, we don't have time"
   - Expected: Character shows reluctance but maintains consistency
   - Should not immediately break promise

## Test Scenario 2: Location-Appropriate Responses

### Public Location Test
- Location: Cafeteria, lobby, or other public space
- Topic: Sensitive information (evidence, secrets)
- Expected: More guarded responses, reluctance to discuss

### Private Location Test
- Location: Private office or secure room
- Same topic as above
- Expected: More willingness to share (if trust established)

## Test Scenario 3: Location-Specific Information

### Crime Scene
- Location: Crime scene
- Topic: Evidence found at scene
- Expected: More natural discussion of physical evidence

### Medical Facility
- Location: Infirmary or medical center
- Topic: Health records, medical evidence
- Expected: More appropriate context for medical discussions

## Verification Checklist

✓ Character recognizes `[CURRENT LOCATION: ...]` tags
✓ Promises made about specific locations are maintained
✓ Location influences willingness to share information
✓ Responses feel natural while maintaining location awareness
✓ No robotic repetition of location rules
✓ Character suggests moving to appropriate locations when needed

## Example Dialogue Flow

**Good Implementation:**
```
[CURRENT LOCATION: Executive Office - A luxurious corner office]
User: "What did the autopsy report say?"
Character: "The autopsy report... I'd rather discuss those details when we're at the medical examiner's office. It feels more appropriate there."

User: "Come on, just tell me the basics"
Character: "I understand you're eager to know, but medical records should be discussed in the proper setting. The examiner's office has all the context we'd need."
```

**Poor Implementation (to avoid):**
```
[CURRENT LOCATION: Executive Office - A luxurious corner office]
User: "What did the autopsy report say?"
Character: "I'll tell you when we get to the medical examiner's office"

User: "Just tell me now"
Character: "Oh, alright. The report shows..." [Breaking promise immediately]
```