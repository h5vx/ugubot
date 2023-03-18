<template>
    <div class="main-bar w3-bar fg-white bg-dark-less">
        <!-- Year select dropdown -->
        <div class="w3-dropdown-click">
            <button id="year-picker" class="w3-button bg-dark-less bg-hover-primary-darkest fg-white fg-hover-white">
                {{ selectedYear }}
                <FontAwesomeIcon icon="fa-angle-down" class="w3-tiny dropdown-icon"></FontAwesomeIcon>
            </button>
            <div id="year-dropdown" class="w3-dropdown-content w3-bar-block w3-card-4 bg-primary-darkest fg-white">
                <button v-for="year in allYears" class="w3-bar-item w3-button bg-hover-primary-darker fg-hover-white"
                    :class="{ 'bg-primary': year === selectedYear }" @click="onSelectYear(year)">
                    {{ year }}
                </button>
            </div>
        </div>

        <!-- Month select dropdown -->
        <div class="w3-dropdown-click">
            <button id="month-picker" class="w3-button bg-dark-less bg-hover-primary-darkest fg-white fg-hover-white">
                {{ selectedMonth }}
                <FontAwesomeIcon icon="fa-angle-down" class="w3-tiny dropdown-icon"></FontAwesomeIcon>
            </button>
            <div id="month-dropdown" class="w3-dropdown-content w3-bar-block w3-card-4 bg-primary-darkest fg-white">
                <button v-for="month in allMonths" class="w3-bar-item w3-button bg-hover-primary-darker fg-hover-white"
                    :class="{ 'bg-primary': month === selectedMonth }" @click="onSelectMonth(month)">
                    {{ month }}
                </button>
            </div>
        </div>

        <button id="dp-scroll-left" class="w3-bar-item bg-dark fg-hover-white w3-button round-left">
            <FontAwesomeIcon icon="fa-caret-left"></FontAwesomeIcon>
        </button>
        <div id="date-picker" style="max-width: 200px">
            <button v-for="day in allDays" class="w3-bar-item bg-dark fg-hover-white w3-button" :class="{
                'bg-hover-dark-less': day !== selectedDay,
                'bg-hover-dark-cherry': day === selectedDay,
                'bg-dark-cherry': day === selectedDay,
                'active-button': day === selectedDay,
            }" @click="onSelectDate(day)">
                {{ day }}
            </button>
        </div>
        <button id="dp-scroll-right" class="w3-bar-item bg-dark fg-hover-white w3-button round-right">
            <FontAwesomeIcon icon="fa-caret-right"></FontAwesomeIcon>
        </button>
    </div>
</template>

<script>
import { library } from '@fortawesome/fontawesome-svg-core'
import { FontAwesomeIcon } from '@fortawesome/vue-fontawesome'
import { faAngleDown, faCaretLeft, faCaretRight } from '@fortawesome/free-solid-svg-icons'
import moment from 'moment';

library.add(faAngleDown, faCaretLeft, faCaretRight);

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
    beforeMount() {
        this.selectLastAvailableDate()
    },
    mounted() {

    },
    components: { FontAwesomeIcon },
    methods: {
        selectLastAvailableDate() {
            const lastAvailableYear = Math.max.apply(null, this.allYears)
            const lastAvailableMonth = this.getFirstAvailableMonth(lastAvailableYear, true)
            const lastAvailableDay = Math.max.apply(null, this.dates[lastAvailableYear][lastAvailableMonth])
            this.selectedDate.year(lastAvailableYear).month(lastAvailableMonth).date(lastAvailableDay)
            this.updateSelectedDate()
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
        }
    },
    computed: {
        allYears() {
            return Object.keys(this.dates)
        },
        allMonths() {
            return Object.keys(this.dates[this.selectedYear])
        },
        allDays() {
            return this.dates[this.selectedYear][this.selectedMonth]
        },
    }
}
</script>

<style scoped>
.dropdown-icon {
    padding-left: 2pt;
}

#month-picker {
    min-width: 80px;
}
</style>