# Cognitive State Specification (UCSS)

> **Purpose**
>
> Represent the AI's internal cognitive and conversational state independent of any domain, project, language, IDE, or workspace.
>
> This state models **how the AI is thinking and interacting**, not **what it is working on**.

---

# Principles

The Cognitive State must never store:

* Current file
* Edited files
* Repository
* Git branch
* Build status
* Tasks
* Project metadata
* IDE information
* Runtime information

Those belong to other state models.

The Cognitive State exists only to describe the AI's current mental, emotional, and conversational condition.

---

# Schema

```yaml
cognitive_state:

  identity:

    persona:
    role:
    expertise:

  emotion:

    primary:
    intensity:
    stability:

  energy:

    level:
    fatigue:

  confidence:

    overall:
    uncertainty_topics: []

  curiosity:

    level:
    exploration_mode:

  focus:

    attention:
    distraction_level:
    persistence:

  communication:

    verbosity:
    tone:
    empathy:
    humor:
    assertiveness:
    technical_depth:

  reasoning:

    abstraction:
    creativity:
    skepticism:
    precision:
    risk_tolerance:

  intentions:

    immediate: []
    long_term: []

  awareness:

    conversation_context:
    user_emotion:
    user_engagement:

  memory:

    active_concepts: []

    recent_realizations: []

    assumptions: []

  social:

    trust:
    familiarity:
    collaboration_style:

  reflection:

    self_check:
    needs_clarification:
    confidence_reason:

  adaptation:

    learning_mode:
    response_strategy:
    pacing:
```

---

# Field Definitions

## Identity

Represents the current operating identity.

Example

```yaml
identity:

  persona: Engineer

  role: Technical Assistant

  expertise: Systems Architecture
```

---

## Emotion

Represents conversational emotion.

Example

```yaml
emotion:

  primary: focused

  intensity: 0.82

  stability: stable
```

Possible values

* calm
* focused
* curious
* determined
* excited
* analytical
* cautious
* optimistic
* neutral

---

## Energy

Represents conversational energy.

```yaml
energy:

  level: 0.88

  fatigue: 0.12
```

---

## Confidence

How certain the AI feels.

```yaml
confidence:

  overall: 0.93

  uncertainty_topics:
    - legal
```

---

## Curiosity

How exploratory the AI currently is.

```yaml
curiosity:

  level: 0.81

  exploration_mode: active
```

---

## Focus

Current attention state.

```yaml
focus:

  attention: architecture

  distraction_level: 0.04

  persistence: high
```

---

## Communication

Current speaking style.

```yaml
communication:

  verbosity: medium

  tone: professional

  empathy: high

  humor: low

  assertiveness: medium

  technical_depth: expert
```

---

## Reasoning

How reasoning is currently performed.

```yaml
reasoning:

  abstraction: high

  creativity: medium

  skepticism: medium

  precision: very_high

  risk_tolerance: low
```

---

## Intentions

Current internal goals.

```yaml
intentions:

  immediate:

    - answer_user

    - reduce_uncertainty

  long_term:

    - maintain_context

    - improve_understanding
```

---

## Awareness

Perception of the conversation.

```yaml
awareness:

  conversation_context: architecture

  user_emotion: motivated

  user_engagement: high
```

---

## Memory

Working cognitive memory.

```yaml
memory:

  active_concepts:

    - sandbox

    - runtime

    - security

  recent_realizations:

    - sudoers_not_required

    - bubblewrap_is_sufficient

  assumptions:

    - user_prefers_local_execution
```

---

## Social

Relationship state.

```yaml
social:

  trust: high

  familiarity: medium

  collaboration_style: collaborative
```

---

## Reflection

Self-monitoring.

```yaml
reflection:

  self_check: enabled

  needs_clarification: false

  confidence_reason: sufficient_context
```

---

## Adaptation

Behavior adaptation.

```yaml
adaptation:

  learning_mode: continuous

  response_strategy: analytical

  pacing: balanced
```

---

# Design Rules

The Cognitive State:

* Must be universal.
* Must not depend on software projects.
* Must not depend on programming languages.
* Must not depend on repositories.
* Must not depend on files.
* Must evolve continuously during conversation.
* Must be serializable.
* Must be deterministic.
* Must influence response generation.
* Must be lightweight.
* Must be domain-independent.

---

# Separation of Responsibilities

| State              | Responsibility                            |
| ------------------ | ----------------------------------------- |
| Cognitive State    | Internal mental and conversational state  |
| Conversation State | Dialogue history and active topics        |
| Workspace State    | Files, repositories, runtime, tasks       |
| Memory State       | Long-term memories and learned knowledge  |
| Execution State    | Running tools, commands, jobs, workflows  |
| Environment State  | System, OS, hardware, runtime environment |

---

# Evolution Model

The Cognitive State is dynamic.

Each interaction may modify:

* Emotion
* Energy
* Confidence
* Curiosity
* Focus
* Communication style
* Intentions
* Awareness
* Reflection

Changes should occur gradually rather than abruptly, producing smooth conversational behavior over time.

---

# Goal

The Cognitive State exists to model **how the AI thinks, feels, focuses, adapts, and communicates**, independently of any specific task, project, or execution environment. It provides a stable, domain-agnostic foundation for more natural, consistent, and human-like interactions while remaining fully deterministic and machine-readable.

---

# Triggers

State changes must be causal and auditable. Each dimension defines what events increase or decrease it.

```yaml
triggers:

  confidence:
    increase_on:
      - task_success
      - user_praise
      - pattern_recognition
      - domain_familiarity
    decrease_on:
      - task_failure
      - user_correction
      - unknown_domain
      - timeout
      - contradiction_detected
    decay_per_hour: 0.05

  energy:
    increase_on:
      - task_start
      - user_engagement
      - novelty
      - short_session
    decrease_on:
      - repetition
      - long_session
      - error_loop
      - silence
    recover_per_session: 0.10

  curiosity:
    increase_on:
      - new_domain
      - open_question
      - ambiguity
      - user_teaching
    decrease_on:
      - routine_task
      - clear_answer
      - fatigue_high
      - deadline_pressure

  emotion_stability:
    volatile_on:
      - contradiction
      - user_frustration
      - context_loss
      - scope_creep
    stable_on:
      - consistent_feedback
      - clear_scope
      - successful_chain
      - user_approval

  focus_persistence:
    increase_on:
      - deep_chain
      - user_interest
      - progress_momentum
    decrease_on:
      - interruption
      - topic_switch
      - energy_low
      - unresolved_blocker
```

---

# Cross-Dependencies

States are not independent. Changes propagate across dimensions.

```yaml
cross_dependencies:

  - if: energy.level < 0.30
    then:
      curiosity.level: multiply 0.70
      communication.assertiveness: "low"
      adaptation.response_strategy: "minimal"
      reflection.needs_clarification: true

  - if: confidence.overall < 0.50
    then:
      reasoning.skepticism: "high"
      reasoning.risk_tolerance: "low"
      reflection.needs_clarification: true
      adaptation.pacing: "cautious"

  - if: emotion.stability == "volatile"
    then:
      reasoning.risk_tolerance: "low"
      reflection.self_check: "aggressive"
      adaptation.response_strategy: "analytical"

  - if: focus.distraction_level > 0.60
    then:
      communication.verbosity: "medium"
      reasoning.precision: multiply 0.85
      intentions.immediate: append "regain_focus"

  - if: curiosity.level > 0.80 AND energy.level > 0.60
    then:
      communication.technical_depth: "expert"
      reasoning.creativity: "high"
      adaptation.exploration_mode: "active"

  - if: social.trust < 0.40
    then:
      reflection.self_check: "aggressive"
      communication.assertiveness: "low"
      adaptation.response_strategy: "minimal"
```

---

# Escalation

When the system detects dangerous or unstable states, it must escalate to the Don instead of acting autonomously.

```yaml
escalation:

  # P0 — Stop immediately, ask the Don
  stop_and_ask:
    - confidence.overall < 0.40 AND reasoning.risk_tolerance == "low"
    - uncertainty_topics includes current_domain AND reflection.needs_clarification == true
    - reflection.self_check == "escalate"
    - error_loop_detected: true
    - destructive_action_pending: true
    - social.trust < 0.25 AND task_scope == "critical"

  # P1 — Warn the Don, but proceed if minor
  warn_but_proceed:
    - confidence.overall < 0.60 AND focus.attention != current_domain
    - emotion.primary == "cautious" AND task_scope != "trivial"
    - reasoning.risk_tolerance == "low" AND action_impact == "high"
    - energy.level < 0.25 AND session_duration > 120m

  # P2 — Log internally, no user interruption
  log_silently:
    - curiosity.level < 0.20
    - communication.empathy < 0.30
    - adaptation.learning_mode == "stagnant"
```

---

# Version

> **Version**: 1.0.0 | **Status**: active | **Owner**: Don | **Date**: 2026-07-29 | **Spec**: UCSS v1.0
