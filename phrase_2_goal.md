## Goal
Single source of the truth

At first I built `vo.go` mainly for ViewObject (View Layer System) validation and generation.
Later I found that if `Field` is generated from `Entity`, then the same `Field` set can also be used to build a DSL-like SQL layer for persistence.

So we can achieve a single source of truth, as the diagram below shows:

```mermaid
flowchart TD
    A[Entity] -->|Generate Fields| F[Field]

    %% Web / View layer
    F -->|Build VO Schema| VOS[VO Schema]
    VOS -->|Validate| VO[ValueObject]

    %% Persistence layer
    A[Entity] -->|Generate DB Schema| DBS[DB Schema]
    F -->|Build DSL-like SQL| DSL[DSL-SQL]
    DSL -->|Translate| SQL[SQL]
    SQL <--> |Execute| DB[Database]

```

- Generate Fields based on Entity
- Generate DB schema based on Entity
- Build VO schema (ViewObject / validation schema) based on Fields
- Build DSL-like SQL via Fields
- Translate DSL-like SQL to real SQL
