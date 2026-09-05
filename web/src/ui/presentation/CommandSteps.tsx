import { Check } from "@phosphor-icons/react/dist/ssr";

export type CommandStage = "details" | "review" | "result";
const stages: ReadonlyArray<{ id: CommandStage; label: string }> = [{ id: "details", label: "Details" }, { id: "review", label: "Review" }, { id: "result", label: "Result" }];

export function CommandSteps({ stage }: Readonly<{ stage: CommandStage }>) {
  const current = stages.findIndex(item => item.id === stage);
  return <ol className="guided-command-steps" aria-label="Progress">{stages.map((item, index) => <li key={item.id} className={index < current ? "complete" : index === current ? "current" : ""} aria-current={item.id === stage ? "step" : undefined}><span className="guided-step-marker" aria-hidden="true">{index < current ? <Check /> : index + 1}</span><span>{item.label}</span></li>)}</ol>;
}
