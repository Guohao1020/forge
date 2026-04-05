"use client";

export function AuroraBackground() {
  return (
    <div className="fixed inset-0 -z-10 aurora-bg">
      <div
        className="absolute inset-0 animate-aurora-1"
        style={{
          background: "radial-gradient(ellipse at 20% 50%, rgba(139, 92, 246, 0.06), transparent 50%)",
        }}
      />
      <div
        className="absolute inset-0 animate-aurora-2"
        style={{
          background: "radial-gradient(ellipse at 80% 20%, rgba(6, 182, 212, 0.04), transparent 50%)",
        }}
      />
      <div
        className="absolute inset-0 animate-aurora-3"
        style={{
          background: "radial-gradient(ellipse at 50% 80%, rgba(59, 130, 246, 0.03), transparent 50%)",
        }}
      />
    </div>
  );
}
