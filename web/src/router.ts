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
      component: () => import("./views/ExecutionList.vue"),
      props: (route) => ({ actionId: route.params.id as string }),
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
      props: (route) => ({ system: route.query.system === "true" }),
    },
    {
      path: "/system",
      name: "system",
      component: () => import("./views/SystemPage.vue"),
    },
  ],
});

export default router;
