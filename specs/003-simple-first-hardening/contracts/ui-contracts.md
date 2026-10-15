# UI Contracts

```ts
type ExperienceMode = "simple" | "expert";

type OperatorUIPreferences = {
  experienceMode: ExperienceMode;
};

type PresentationStatus = {
  title: string;
  explanation: string;
  attention: boolean;
  tone: "neutral" | "positive" | "warning" | "danger" | "unknown";
  nextAction?: { label: string; href: string };
  evidence?: ReadonlyArray<{ label: string; value: string }>;
};
```

- Presentation mode may alter copy, density, navigation, and evidence visibility only.
- Every hidden Simple-view fact remains reachable through details or Expert view unless it is an unreleased capability.
- Unknown outcomes and urgent errors remain visible in both modes.
