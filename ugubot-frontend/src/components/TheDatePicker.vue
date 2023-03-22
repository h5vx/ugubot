<template>
    <div class="main-bar w3-bar fg-white bg-dark-less w3-card-2">
        <!-- Year select dropdown -->
        <div class="w3-dropdown-click">
            <button ref="yearPicker" class="w3-button bg-dark-less bg-hover-primary-darkest fg-white fg-hover-white">
                {{ selectedYear }}
                <FontAwesomeIcon icon="fa-angle-down" class="w3-tiny dropdown-icon"></FontAwesomeIcon>
            </button>
            <div ref="yearDropdown" class="w3-dropdown-content w3-bar-block w3-card-4 bg-primary-darkest fg-white">
                <button v-for="year in allYears" class="w3-bar-item w3-button bg-hover-primary-darker fg-hover-white"
                    :class="{ 'bg-primary': year === selectedYear }" @click="onSelectYear(year)">
                    {{ year }}
                </button>
            </div>
        </div>

        <!-- Month select dropdown -->
        <div class="w3-dropdown-click">
            <button ref="monthPicker" class="w3-button bg-dark-less bg-hover-primary-darkest fg-white fg-hover-white">
                {{ selectedMonth }}
                <FontAwesomeIcon icon="fa-angle-down" class="w3-tiny dropdown-icon"></FontAwesomeIcon>
            </button>
            <div ref="monthDropdown" class="w3-dropdown-content w3-bar-block w3-card-4 bg-primary-darkest fg-white">
                <button v-for="month in allMonths" class="w3-bar-item w3-button bg-hover-primary-darker fg-hover-white"
                    :class="{ 'bg-primary': month === selectedMonth }" @click="onSelectMonth(month)">
                    {{ month }}
                </button>
            </div>
        </div>

        <button id="scroll-left" ref="scrollLeft" class="w3-bar-item bg-dark fg-hover-white w3-button round-left">
            <FontAwesomeIcon icon="fa-caret-left"></FontAwesomeIcon>
        </button>
        <div id="day-picker" ref="dayPicker" style="max-width: 200px">
            <button v-for="day in allDays" class="w3-bar-item bg-dark fg-hover-white w3-button" :class="{
                'bg-hover-dark-less': day !== selectedDay,
                'bg-hover-dark-cherry': day === selectedDay,
                'bg-dark-cherry': day === selectedDay,
                'active-button': day === selectedDay,
            }" @click="onSelectDate(day)">
                {{ day }}
            </button>
            <button v-if="selectedDay && !haveCurrentDate" class="w3-bar-item fg-hover-white w3-button btn-plus"
                @click="addTodayDate">
                <FontAwesomeIcon icon="fa-plus"></FontAwesomeIcon>
            </button>
        </div>
        <button id="scroll-right" ref="scrollRight" class="w3-bar-item bg-dark fg-hover-white w3-button round-right">
            <FontAwesomeIcon icon="fa-caret-right"></FontAwesomeIcon>
        </button>
    </div>
</template>

<script>
import { library } from '@fortawesome/fontawesome-svg-core';
import { faAngleDown, faCaretLeft, faCaretRight, faPlus } from '@fortawesome/free-solid-svg-icons';
import { FontAwesomeIcon } from '@fortawesome/vue-fontawesome';
import moment from 'moment-timezone';

library.add(faAngleDown, faCaretLeft, faCaretRight, faPlus);

export default {
    props: {
        dates: {
            type: Object,
            required: true,
        }
    },
    emits: ["dateSelected"],
    data() {
        return {
            selectedDate: moment(),
            selectedYear: null,
            selectedMonth: null,
            selectedDay: null,
        }
    },
    watch: {
        dates() {
            this.selectLastAvailableDate()
        },
    },
    beforeMount() {
        this.selectLastAvailableDate()
    },
    components: { FontAwesomeIcon },
    methods: {
        selectLastAvailableDate() {
            if (!this.dates) return
            if ("..." in this.dates) return

            const lastAvailableYear = Math.max.apply(null, this.allYears)
            const lastAvailableMonth = this.getFirstAvailableMonth(lastAvailableYear, true)
            const lastAvailableDay = Math.max.apply(null, this.dates[lastAvailableYear][lastAvailableMonth])
            this.selectedDate.year(lastAvailableYear).month(lastAvailableMonth).date(lastAvailableDay)
            this.updateSelectedDate()
            this.$emit('dateSelected', this.selectedDate)
        },
        addTodayDate() {
            const [year, month, day] = moment().format("YYYY/MMM/DD").split("/")

            if (!(year in this.dates)) {
                this.dates[year] = {[month]: [day]}
            } else if (!(month in this.dates[year])) {
                this.dates[year][month] = [day]
            } else if (!this.dates[year][month].includes(day)) {
                this.dates[year][month].push(day)
            }

            this.selectLastAvailableDate()
        },
        updateSelectedDate() {
            this.selectedYear = this.selectedDate.format("YYYY")
            this.selectedMonth = this.selectedDate.format("MMM")
            this.selectedDay = this.selectedDate.format("DD")
        },
        onSelectYear(year) {
            if (year === this.selectedYear) return
            this.selectedDate.year(year).month(this.getFirstAvailableMonth(year)).date(this.getFirstAvailableDay())
            this.updateSelectedDate()
        },
        onSelectMonth(month) {
            if (month === this.selectedMonth) return
            this.selectedDate.month(month).date(this.getFirstAvailableDay())
            this.updateSelectedDate()
        },
        onSelectDate(day) {
            if (day === this.selectedDay) return
            this.selectedDay = day
            this.selectedDate.date(day)
            this.$emit('dateSelected', this.selectedDate)
        },
        getFirstAvailableMonth(year, reverse = false) {
            if (!this.dates) return
            if ("..." in this.dates) return

            let months = ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Dec"]

            if (reverse) {
                months = months.reverse()
            }

            for (let month of months) {
                if (this.dates[year][month]) {
                    return month
                }
            }
        },
        getFirstAvailableDay() {
            return this.dates[this.selectedYear][this.selectedMonth][0]
        },
        UIUpdateScrollButtons() {
            const dayPicker = this.$refs.dayPicker

            if (!dayPicker) {
                return
            }

            const dayPickerScrollLeft = this.$refs.scrollLeft
            const dayPickerScrollRight = this.$refs.scrollRight
            const scrollPercentage = 100 * dayPicker.scrollLeft / (dayPicker.scrollWidth - dayPicker.clientWidth)

            const leftDisabled = (scrollPercentage == 0 || dayPicker.scrollWidth <= dayPicker.clientWidth)
            const rightDisabled = (scrollPercentage >= 99 || dayPicker.scrollWidth <= dayPicker.clientWidth)

            dayPickerScrollLeft.classList.toggle("w3-opacity-max", leftDisabled)
            dayPickerScrollRight.classList.toggle("w3-opacity-max", rightDisabled)
        },
        UIUpdateDayPickerWidth() {
            const dayPicker = this.$refs.dayPicker

            if (!dayPicker) {
                return
            }

            const dayPickerScrollLeft = this.$refs.scrollLeft
            const dayPickerScrollRight = this.$refs.scrollRight
            const dpScrollLeftBtnRect = dayPickerScrollLeft.getBoundingClientRect();
            const dpScrollRightBtnRect = dayPickerScrollRight.getBoundingClientRect();
            const docWidth = document.body.getBoundingClientRect().width;
            const dpNewWidth = docWidth - dpScrollLeftBtnRect.x - dpScrollLeftBtnRect.width - dpScrollRightBtnRect.width - 10;

            dayPicker.style.maxWidth = dpNewWidth + "px";
        },
    },
    computed: {
        allYears() {
            if (!this.dates) return []
            if ("..." in this.dates) return

            return Object.keys(this.dates)
        },
        allMonths() {
            if (!this.dates) return []
            if ("..." in this.dates) return

            return Object.keys(this.dates[this.selectedYear])
        },
        allDays() {
            if (!this.dates) return []
            if ("..." in this.dates) return

            return this.dates[this.selectedYear][this.selectedMonth]
        },
        haveCurrentDate() {
            if (!this.dates) return false
            const [year, month, day] = moment().format("YYYY/MMM/DD").split("/")

            return (
                year in this.dates
                && month in this.dates[year]
                && this.dates[year][month].includes(day)
            )
        },
    },
    mounted() {
        const dayPicker = this.$refs.dayPicker
        const dayPickerScrollLeft = this.$refs.scrollLeft
        const dayPickerScrollRight = this.$refs.scrollRight

        const scroller = (direction) => {
            return (e) => {
                var scrollSize = (dayPicker.offsetWidth / 2);

                if (dayPicker.offsetWidth < 200) {
                    scrollSize = dayPicker.offsetWidth;
                }

                dayPicker.scroll({
                    left: dayPicker.scrollLeft + scrollSize * direction,
                    behavior: "smooth",
                });
            }
        }

        dayPicker.addEventListener("scroll", this.UIUpdateScrollButtons);

        dayPickerScrollLeft.addEventListener("click", scroller(-1));
        dayPickerScrollRight.addEventListener("click", scroller(1));

        // Date picker auto resize
        const main = document.getElementById("main");

        // updateDatePickerWidth();
        new ResizeObserver(this.UIUpdateDayPickerWidth).observe(main)

        // Year & month dropdowns
        const yearPickButton = this.$refs.yearPicker
        const monthPickButton = this.$refs.monthPicker
        const yearDropDown = this.$refs.yearDropdown
        const monthDropDown = this.$refs.monthDropdown

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

        this.UIUpdateScrollButtons();
        this.UIUpdateDayPickerWidth();
        new ResizeObserver(this.UIUpdateScrollButtons).observe(main);
    },
    updated() {
        this.UIUpdateScrollButtons()
        this.UIUpdateDayPickerWidth()
    }
}
</script>

<style scoped>
.main-bar {
    background-color: #0c0e15;
    position: sticky;
    top: 0;
    overflow: visible;
    z-index: 7;
}

.btn-plus {
    background-color: #172d2e;
}

.btn-plus:hover {
    background-color: #1e3d3f !important;
}

.dropdown-icon {
    padding-left: 2pt;
}

#month-picker {
    min-width: 80px;
}

#day-picker {
    overflow: auto;
    overflow-x: hidden;
    white-space: nowrap;
    font-size: 0;

    display: block;
    float: left;
}

#day-picker>button {
    display: inline-block;
    float: none;
}

#day-picker .active-button {
    position: sticky;
    left: 0;
    right: 0;
}

#scroll-left,
#scroll-right,
#day-picker>button {
    font-size: 13px;
    height: 34px;
}

#day-picker,
#scroll-left,
#scroll-right {
    margin-top: 2px;
    background: linear-gradient(180deg, rgba(35, 42, 62, 1) 0%, rgba(7, 8, 12, 1) 100%);
}
</style>