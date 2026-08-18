import { notFound } from "next/navigation";

// Administrative UI is intentionally not shipped until privileged sessions and
// a separate operator authorization boundary exist. This is deny-by-default.
export default function AdminPage() { notFound(); }
