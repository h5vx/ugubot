def pluralize(n, singular, plural, plural2):
    if n == 1:
        return singular
    elif n % 10 == 1 and n % 100 != 11:
        return plural
    elif n % 10 in [2, 3, 4] and n % 100 not in [12, 13, 14]:
        return plural2
    else:
        return plural
