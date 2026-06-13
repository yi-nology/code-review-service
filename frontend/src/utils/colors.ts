export function getScoreColor(score: number): string {
  if (score >= 80) return '#159570'
  if (score >= 60) return '#d98616'
  return '#d84b5b'
}

export function getScoreGrade(score: number): string {
  if (score >= 90) return 'A'
  if (score >= 80) return 'B'
  if (score >= 70) return 'C'
  if (score >= 60) return 'D'
  return 'F'
}
