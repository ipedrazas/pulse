import { Toaster } from 'sonner'

export function ToastProvider() {
  return (
    <Toaster
      position="bottom-right"
      toastOptions={{
        className: 'bg-gray-900 text-white border border-gray-700',
        duration: 4000,
      }}
      theme="dark"
    />
  )
}
