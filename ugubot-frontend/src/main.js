import { createApp } from 'vue'
import App from './App.vue'

import './assets/main.css'

createApp(App).mount('#app')

function setupResizeHandle(el) {
    const resizable = el.parentElement;
    const minWidth = Number(el.attributes["min-width"].value);
    const maxWidth = Number(el.attributes["max-width"].value);
    const main = document.getElementById("main");
    const body = document.getElementsByTagName("body")[0];

    const preventTextSelection = () => {
        body.style.userSelect = "none";
    }

    const allowTextSelection = () => {
        body.style.userSelect = null;
    }

    const onResize = (e) => {
        if (e.pageX > maxWidth || e.pageX < minWidth) {
            return;
        }

        resizable.style.width = e.pageX + "px";
        main.style.marginLeft = e.pageX + "px";
    }

    const stopResize = (e) => {
        window.removeEventListener("mousemove", onResize);
        allowTextSelection();
        localStorage.sidebarWidth = resizable.style.width;
    }

    const startResize = (e) => {
        window.addEventListener("mousemove", onResize);
        window.addEventListener("mouseup", stopResize);
        preventTextSelection();
    }

    el.addEventListener("mousedown", startResize);

    if ("sidebarWidth" in localStorage) {
        resizable.style.width = localStorage.sidebarWidth;
        main.style.marginLeft = localStorage.sidebarWidth;
    }
}

document.addEventListener("DOMContentLoaded", (e) => {
    // Sidebar resizer
    const resizeHandles = document.getElementsByClassName("resizer-handle");

    for (const el of resizeHandles) {
        setupResizeHandle(el);
    }

    const menuButton = document.getElementById("menu-button");
    const sidebar = document.getElementById("sidebar");
    const sidebarCloseButton = document.getElementById("sidebar-close-button");

    menuButton.addEventListener("click", (e) => {
        sidebar.style.display = "block";
    });

    sidebarCloseButton.addEventListener("click", (e) => {
        sidebar.style.display = "none";
    });
})