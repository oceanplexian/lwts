# Accessibility

## Keyboard Navigation
- Press Tab repeatedly. Focus should move through interactive elements in a logical order.
- Focus a card and press Enter. The detail modal should open.
- With a modal open, press Escape. The modal should close.

## ARIA
- Column containers should have appropriate ARIA roles.
- Drag handles should have aria-labels.
- Modals should have `role="dialog"` and `aria-modal="true"`.
- Cards should have aria-labels summarizing their key info.
- Column headers should have aria-labels including card count.

## Screen Reader
- When a card is created via SSE, an `aria-live="polite"` region should be updated.

## Color & Motion
- Priority indicators should use different shapes, not just colors.
- With "prefers-reduced-motion" enabled, animated elements should have `transition-duration: 0`.
