import { createRouter, createWebHistory } from "vue-router";

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: "/",
      name: "dashboard",
      component: () => import("./views/Dashboard.vue"),
    },
    {
      path: "/actions",
      name: "actions",
      component: () => import("./views/ActionList.vue"),
    },
    {
      path: "/actions/:id",
      name: "action-detail",
      component: () => import("./views/ActionDetail.vue"),
    },
    {
      path: "/actions/:id/executions",
      name: "action-executions",
      component: () => import("./views/ActionExecutionsPage.vue"),
    },
    {
      path: "/actions/:id/executions/:hid",
      name: "log-view",
      component: () => import("./views/LogView.vue"),
    },
    {
      path: "/executions",
      name: "executions",
      component: () => import("./views/ExecutionList.vue"),
    },
    {
      path: "/system",
      name: "system",
      component: () => import("./views/SystemPage.vue"),
    },
  ],
});

export default router;
