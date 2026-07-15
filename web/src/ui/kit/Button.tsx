import type { ComponentProps } from 'react'
import { cx } from './cx'
import styles from './Button.module.css'

export function Button({ className, ...props }: ComponentProps<'button'>) {
  return <button className={cx(styles.button, className)} {...props} />
}
