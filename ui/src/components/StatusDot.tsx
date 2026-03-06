interface StatusDotProps {
  status: string;
  className?: string;
}

export function StatusDot({ status, className = '' }: StatusDotProps) {
  const color = status === 'online' ? 'bg-green-400' : 'bg-gray-500';
  return (
    <span
      className={`inline-block h-2.5 w-2.5 rounded-full ${color} ${className}`}
      title={status}
    />
  );
}
