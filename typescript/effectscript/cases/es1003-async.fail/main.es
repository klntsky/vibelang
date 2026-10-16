// expect: ES1003
// async is not part of EffectScript: a call already waits (Chapter 10).

export async function load() {
  return 1
}

export const loadAll = async () => [await load()]

export class Repo {
  async find(id: string) {
    return id
  }
}
