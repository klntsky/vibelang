// effeck checks .ts and .effeck files; it sees a .vue file only through this
// declaration. Vite compiles the component itself with @vitejs/plugin-vue.
declare module "*.vue" {
  import type { DefineComponent } from "vue"
  const component: DefineComponent
  export default component
}
