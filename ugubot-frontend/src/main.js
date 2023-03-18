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

    // Date picker scroll
    const datePicker = document.getElementById("date-picker");
    const datePickerScrollLeftButton = document.getElementById("dp-scroll-left");
    const datePickerScrollRightButton = document.getElementById("dp-scroll-right");

    const scroller = (direction) => {
        return (e) => {
            var scrollSize = (datePicker.offsetWidth / 2);

            if (datePicker.offsetWidth < 200) {
                scrollSize = datePicker.offsetWidth;
            }

            datePicker.scroll({
                left: datePicker.scrollLeft + scrollSize * direction,
                behavior: "smooth",
            });
        }
    }

    const dpScrollUpdateButtons = () => {
        const scrollPercentage = 100 * datePicker.scrollLeft / (datePicker.scrollWidth - datePicker.clientWidth);

        if (scrollPercentage > 99) {
            datePickerScrollRightButton.classList.add("w3-opacity-max");
        } else if (scrollPercentage == 0) {
            datePickerScrollLeftButton.classList.add("w3-opacity-max");
        }

        if (scrollPercentage > 0) {
            datePickerScrollLeftButton.classList.remove("w3-opacity-max");
        } 

        if (scrollPercentage <= 99) {
            datePickerScrollRightButton.classList.remove("w3-opacity-max");
        }
    }

    datePicker.addEventListener("scroll", dpScrollUpdateButtons);

    datePickerScrollLeftButton.addEventListener("click", scroller(-1));
    datePickerScrollRightButton.addEventListener("click", scroller(1));

    // Date picker auto resize
    const main = document.getElementById("main");

    const updateDatePickerWidth = () => {
        const dpScrollLeftBtnRect = datePickerScrollLeftButton.getBoundingClientRect();
        const dpScrollRightBtnRect = datePickerScrollRightButton.getBoundingClientRect();
        const docWidth = document.body.getBoundingClientRect().width;
        const dpNewWidth = docWidth - dpScrollLeftBtnRect.x - dpScrollLeftBtnRect.width - dpScrollRightBtnRect.width - 10;
        datePicker.style.maxWidth = dpNewWidth + "px";
    }

    // updateDatePickerWidth();
    new ResizeObserver(updateDatePickerWidth).observe(main);

    // Year & month dropdowns
    const yearPickButton = document.getElementById("year-picker");
    const monthPickButton = document.getElementById("month-picker");
    const yearDropDown = document.getElementById("year-dropdown");
    const monthDropDown = document.getElementById("month-dropdown");

    const showYearDropDown = () => { yearDropDown.classList.add("w3-show"); };
    const hideYearDropDown = () => { yearDropDown.classList.remove("w3-show"); };
    const showMonthDropDown = () => { monthDropDown.classList.add("w3-show"); };
    const hideMonthDropDown = () => { monthDropDown.classList.remove("w3-show"); };

    yearDropDown.addEventListener("mouseleave", hideYearDropDown);
    monthDropDown.addEventListener("mouseleave", hideMonthDropDown);

    yearPickButton.addEventListener("mouseleave", () => {
        if (!yearDropDown.matches(":hover")) {
            hideYearDropDown();
        }
    });

    monthPickButton.addEventListener("mouseleave", () => {
        if (!monthDropDown.matches(":hover")) {
            hideMonthDropDown();
        }
    });

    yearPickButton.addEventListener("click", showYearDropDown);
    monthPickButton.addEventListener("click", showMonthDropDown);

    setTimeout(() => {
        dpScrollUpdateButtons();
        updateDatePickerWidth();
    }, 500);
    new ResizeObserver(dpScrollUpdateButtons).observe(main);
})