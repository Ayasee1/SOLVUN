import * as React from 'react'
import { AlertCircle } from 'lucide-react'
import { cn } from '@/lib/utils'

function Field({ className, ...props }: React.ComponentProps<'div'>) {
  return (
    <div
      data-slot='field'
      className={cn('grid content-start gap-2', className)}
      {...props}
    />
  )
}

function FieldGroup({ className, ...props }: React.ComponentProps<'div'>) {
  return (
    <div
      data-slot='field-group'
      className={cn('grid gap-5', className)}
      {...props}
    />
  )
}

function FieldLabel({ className, ...props }: React.ComponentProps<'label'>) {
  return (
    <label
      data-slot='field-label'
      className={cn(
        'flex items-center gap-1.5 text-sm leading-none font-medium text-foreground select-none peer-disabled:cursor-not-allowed peer-disabled:opacity-50',
        className
      )}
      {...props}
    />
  )
}

function FieldDescription({
  className,
  ...props
}: React.ComponentProps<'p'>) {
  return (
    <p
      data-slot='field-description'
      className={cn(
        'text-muted-foreground text-xs leading-relaxed',
        className
      )}
      {...props}
    />
  )
}

function FieldError({ className, children, ...props }: React.ComponentProps<'p'>) {
  if (!children) return null

  return (
    <p
      data-slot='field-error'
      role='alert'
      className={cn(
        'text-destructive flex items-start gap-1.5 text-xs leading-relaxed',
        className
      )}
      {...props}
    >
      <AlertCircle className='mt-0.5 size-3.5 shrink-0' />
      <span>{children}</span>
    </p>
  )
}

export { Field, FieldGroup, FieldLabel, FieldDescription, FieldError }
