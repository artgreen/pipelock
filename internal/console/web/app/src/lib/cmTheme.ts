import { EditorView } from '@codemirror/view'
import { HighlightStyle, syntaxHighlighting } from '@codemirror/language'
import { tags as t } from '@lezer/highlight'
import type { Extension } from '@codemirror/state'

// Cyber-terminal CodeMirror theme: near-black bg, neon-green accents.
const base = EditorView.theme(
  {
    '&': {
      color: '#c8e6c8',
      backgroundColor: '#0a0c0a',
      fontSize: '13px',
      height: '100%',
    },
    '.cm-content': {
      fontFamily: 'var(--font-mono)',
      caretColor: '#39ff14',
    },
    '.cm-cursor, .cm-dropCursor': { borderLeftColor: '#39ff14' },
    '&.cm-focused .cm-selectionBackground, .cm-selectionBackground, .cm-content ::selection': {
      backgroundColor: 'rgba(57,255,20,0.18)',
    },
    '.cm-gutters': {
      backgroundColor: '#0a0c0a',
      color: '#3a5a3a',
      border: 'none',
      borderRight: '1px solid #1a2b1a',
    },
    '.cm-activeLineGutter': { backgroundColor: 'rgba(57,255,20,0.06)', color: '#39ff14' },
    '.cm-activeLine': { backgroundColor: 'rgba(57,255,20,0.04)' },
    '.cm-lineNumbers .cm-gutterElement': { padding: '0 0.5rem 0 0.75rem' },
    '.cm-scroller': { fontFamily: 'var(--font-mono)' },
    '.cm-matchingBracket, &.cm-focused .cm-matchingBracket': {
      backgroundColor: 'rgba(57,255,20,0.2)',
      color: '#39ff14',
    },
  },
  { dark: true },
)

const highlight = HighlightStyle.define([
  { tag: [t.definition(t.propertyName), t.propertyName, t.keyword], color: '#39ff14' },
  { tag: [t.string], color: '#a8e6a8' },
  { tag: [t.number, t.bool, t.null, t.atom], color: '#ffcc00' },
  { tag: [t.comment], color: '#4a6b4a', fontStyle: 'italic' },
  { tag: [t.meta], color: '#7aa87a' },
  { tag: [t.punctuation, t.separator], color: '#4a6b4a' },
])

export const cyberCodeMirror: Extension = [base, syntaxHighlighting(highlight)]
