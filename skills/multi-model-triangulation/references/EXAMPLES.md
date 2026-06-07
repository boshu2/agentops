# Triangulation Examples

Real examples of multi-model triangulation in action.

---

## Example 1: Architecture Decision

### The Question

"Should we use GraphQL or REST for our new API?"

### Prompt Sent to All Models

```
I'm choosing between GraphQL and REST for a new API:

Context:
- Team: 4 developers, familiar with REST, new to GraphQL
- Clients: Web app + mobile app
- Data: Complex relationships (users → posts → comments → reactions)
- Scale: 10K DAU initially, targeting 100K

Evaluate on: Complexity, Performance, Developer Experience, Long-term maintenance

Recommend ONE with reasoning.
```

### Responses

**Claude:**
> REST for your situation. Your team knows it, and GraphQL's benefits (flexible queries, reduced over-fetching) don't outweigh the learning curve for a 4-person team. Complex relationships can be handled with well-designed REST endpoints + sparse fieldsets. GraphQL shines when you have many different clients with very different data needs—you have two similar clients.

**Gemini:**
> GraphQL. Your complex relationships (users→posts→comments→reactions) are exactly where GraphQL excels. Mobile apps especially benefit from fetching exactly what they need. Yes, learning curve, but it pays off quickly. With 10K→100K growth, you'll appreciate not maintaining N+1 REST endpoints.

**Codex:**
> REST with some GraphQL patterns. Use REST for simple CRUD, but consider a single `/graphql` endpoint for complex queries. This gives you familiarity (REST) while allowing flexible queries where needed. Pragmatic middle ground.

### Synthesis

```markdown
## Triangulation Results: GraphQL vs REST

### Consensus (High Confidence)
- Complex relationships are a consideration favoring GraphQL
- Team familiarity with REST is real friction for GraphQL
- Mobile optimization matters for both approaches

### Divergence
| Aspect | Claude | Gemini | Codex |
|--------|--------|--------|-------|
| Recommendation | REST | GraphQL | Hybrid |
| Key factor | Team skills | Data complexity | Pragmatism |

**Why they differ:**
- Claude weighted team productivity higher
- Gemini weighted technical fit higher
- Codex sought middle ground

### Unique Insights
- **Claude:** "Sparse fieldsets" can solve over-fetching in REST
- **Gemini:** N+1 endpoint problem at scale
- **Codex:** Hybrid approach possible

### Synthesized Recommendation
**Start with REST, add GraphQL incrementally.**
- Build core CRUD with REST (team velocity)
- Add single GraphQL endpoint for complex mobile queries
- Revisit in 6 months with usage data

### Confidence Level
Medium — genuine trade-off, no clear winner
```

---

## Example 2: Code Review

### The Code

```python
def process_payment(user_id, amount, card_token):
    user = db.query(f"SELECT * FROM users WHERE id = {user_id}")
    if user.balance >= amount:
        stripe.charge(card_token, amount)
        db.execute(f"UPDATE users SET balance = balance - {amount} WHERE id = {user_id}")
        return {"success": True}
    return {"error": "Insufficient funds"}
```

### Responses

**All three models agreed on:**
1. SQL injection vulnerability (critical)
2. Race condition between balance check and update (critical)
3. No error handling for Stripe failures (important)

**Claude uniquely noted:**
> The function charges the card even if the balance check passes—this seems backwards. If user has sufficient balance, why charge the card? Business logic may be confused.

**Gemini uniquely noted:**
> Stripe charge should be idempotent. If this retries, user could be double-charged. Need idempotency key.

**Codex uniquely noted:**
> Return types inconsistent—dict with "success" vs dict with "error". Should use consistent response structure.

### Synthesis

```markdown
## Triangulation Results: Payment Code Review

### Consensus (Critical Issues)
1. SQL injection via string formatting
2. Race condition (TOCTOU between check and update)
3. Missing Stripe error handling

### Unique Insights Combined

**Synthesized Fix:**
```python
def process_payment(user_id: int, amount: Decimal, card_token: str) -> PaymentResult:
    try:
        with db.transaction():
            # Parameterized query, row-level lock
            user = db.query(
                "SELECT * FROM users WHERE id = %s FOR UPDATE",
                (user_id,)
            )
            if user.balance < amount:
                return PaymentResult(success=False, error="Insufficient funds")

            # Idempotency key prevents double-charge
            idempotency_key = f"payment-{user_id}-{uuid4()}"
            stripe.charge(
                card_token,
                amount,
                idempotency_key=idempotency_key
            )

            db.execute(
                "UPDATE users SET balance = balance - %s WHERE id = %s",
                (amount, user_id)
            )
            return PaymentResult(success=True)
    except stripe.CardError as e:
        return PaymentResult(success=False, error=str(e))
```

### Confidence Level
High — all models found the same critical issues
```

---

## Example 3: Idea Triangulation

### Ideas Evaluated

Three feature ideas for a task management app:
1. AI-powered task prioritization
2. Calendar sync with Google/Outlook
3. Team workload visualization

### Synthesis

| Idea | Claude | Gemini | Codex | Avg |
|------|--------|--------|-------|-----|
| AI prioritization | 7/10 | 8/10 | 6/10 | 7.0 |
| Calendar sync | 9/10 | 8/10 | 9/10 | 8.7 |
| Workload viz | 6/10 | 7/10 | 8/10 | 7.0 |

**Consensus:** Calendar sync is the clear winner—high utility, proven pattern, lower risk.

**Divergence:** AI prioritization split opinions. Claude and Codex worried about "AI magic" disappointing users. Gemini more optimistic about LLM capabilities.

**Decision:** Build calendar sync first, prototype AI prioritization quietly.

---

## When to Triangulate

| Scenario | Value | Models to Use |
|----------|-------|---------------|
| High-stakes architecture | High | Claude + Gemini + Codex |
| Security review | Very High | All available |
| Code review (routine) | Medium | 1-2 models |
| Quick question | Low | Don't bother |
| Creative brainstorming | High | Include Grok for unconventional takes |

**Rule of thumb:** If the decision is hard to reverse or high-impact, triangulate.
