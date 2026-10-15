"use client";

import { Eye, SlidersHorizontal } from "@phosphor-icons/react";

import { useExperienceMode } from "@/features/console/ExperienceModeBoundary";

export function ExperienceModeSwitch() {
  const { mode, setMode } = useExperienceMode();
  const expert = mode === "expert";
  return (
    <button
      className="experience-mode-switch"
      type="button"
      aria-label={expert ? "Switch to Simple view" : "Switch to Expert view"}
      aria-pressed={expert}
      onClick={() => setMode(expert ? "simple" : "expert")}
    >
      {expert ? <Eye aria-hidden="true" /> : <SlidersHorizontal aria-hidden="true" />}
      <span>{expert ? "Simple view" : "Expert view"}</span>
    </button>
  );
}
