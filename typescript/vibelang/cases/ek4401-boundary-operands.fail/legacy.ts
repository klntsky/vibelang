export function sortTitles(titles: ReadonlyArray<{ title: string }>): Array<string> {
  return titles.map((todo) => todo.title).sort()
}

export interface Shelf {
  titles(): Array<string>
}
