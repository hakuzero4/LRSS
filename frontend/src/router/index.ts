import { createRouter, createWebHashHistory } from "vue-router";
import AppLayout from "@/layouts/AppLayout.vue";
import ReaderView from "@/views/ReaderView.vue";

export const router = createRouter({
  history: createWebHashHistory(),
  routes: [
    {
      path: "/",
      component: AppLayout,
      children: [
        {
          path: "",
          name: "reader",
          component: ReaderView,
        },
      ],
    },
  ],
});
