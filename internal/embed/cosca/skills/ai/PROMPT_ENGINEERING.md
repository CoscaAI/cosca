> **Version**: 1.0.0 | **Status**: active | **Owner**: AI Chief | **Last Updated**: 2026-07-23
>
> # PROMPT ENGINEERING SKILL
>
> ## Description
> Use this skill to design, optimize, and test AI prompts following prompt engineering best practices. Covers system prompts, few-shot learning, chain-of-thought, structured output, and prompt versioning.
>
> ## Inputs
> | Input | Required | Description |
> |-------|----------|-------------|
> | task_description | Yes | What the prompt should accomplish |
> | prompt_type | Yes | `system`, `user`, `few-shot`, `chain-of-thought`, `structured-output` |
> | model | Yes | Target model (e.g., `gpt-4`, `claude-3`, `llama-3`) |
> | constraints | No | Specific constraints (format, length, tone) |
>
> ## Outputs
> | Output | Description |
> |--------|-------------|
> | Optimized prompt | Ready-to-use prompt |
> | Test results | Prompt effectiveness metrics |
> | Version history | Prompt version tracking |
>
> ## Process
> 1. Analyze task requirements
> 2. Design prompt structure (role, context, task, constraints, output format)
> 3. Apply prompt patterns (persona, chain-of-thought, few-shot)
> 4. Add explicit output formatting instructions
> 5. Include edge case handling
> 6. Test prompt with sample inputs
> 7. Iterate based on results
> 8. Version and document the prompt
>
> ## Prompt Patterns
> - Role Assignment: "You are a senior software architect..."
> - Chain of Thought: "Let's think step by step..."
> - Few-Shot: Provide 2-3 examples
> - Structured Output: JSON schema for response format
> - Constraint Injection: Explicit do's and don'ts
> - Context Windowing: Prioritize relevant context
>
> ## Success Criteria
> - [ ] Prompt produces consistent, high-quality results
> - [ ] Output format is reliable and parseable
> - [ ] Edge cases handled gracefully
> - [ ] Prompt tested with multiple inputs
> - [ ] Version documented in prompt registry
>
> ## Related
> - [AI Chief](../../departments/ai/SKILL.md)
> - [Provider Chief](../../departments/provider/SKILL.md)
> - [Provider Discovery](./PROVIDER_DISCOVERY.md)
> - [Embedding Pipeline](./EMBEDDING_PIPELINE.md)
