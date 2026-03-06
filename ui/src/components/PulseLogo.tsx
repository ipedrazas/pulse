export function PulseLogo({ className = "h-7 w-7" }: { className?: string }) {
  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      viewBox="0 0 32 32"
      className={className}
    >
      <rect width="32" height="32" rx="6" fill="#1a2332" />
      <polyline
        points="3,18 9,18 12,8 16,24 20,12 23,18 29,18"
        fill="none"
        stroke="#4ade80"
        strokeWidth="2.5"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </svg>
  )
}
