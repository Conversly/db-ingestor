Queue: [DS1, DS2, DS3, DS4, DS5, DS6, DS7, DS8, DS9, DS10]
Workers: [W1, W2, W3]

Time 0: W1→DS1, W2→DS2, W3→DS3 (processing)
        Queue: [DS4, DS5, DS6, DS7, DS8, DS9, DS10] (waiting)

Time 1: W1 finishes DS1, takes DS4
        W2→DS2, W3→DS3, W1→DS4
        Queue: [DS5, DS6, DS7, DS8, DS9, DS10]

Time 2: W2 finishes DS2, takes DS5
        W1→DS4, W3→DS3, W2→DS5
        Queue: [DS6, DS7, DS8, DS9, DS10]

... continues until all done