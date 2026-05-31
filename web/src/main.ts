import { createApp } from "vue";
import App from "./App.vue";
import router from "./router";
import { boltIcon } from "./icon";
import "./style.css";

const link =
  (document.querySelector('link[rel="icon"]') as HTMLLinkElement) ||
  Object.assign(document.createElement("link"), { rel: "icon" });
link.href = "data:image/svg+xml," + encodeURIComponent(boltIcon);
document.head.appendChild(link);

const app = createApp(App);
app.use(router).mount("#app");
