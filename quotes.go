package janet

func badJanetQuote () {

    quotes := []string{
        "https://66.media.tumblr.com/02118197693f204d5a7e95b92075cd83/tumblr_og4ljpvuMl1u4ypbyo1_500.gif",
        "https://66.media.tumblr.com/6a88772a79bcec4630895ff6435744cf/tumblr_p312bp58oB1wd0zf4o2_400.gif",
        "https://66.media.tumblr.com/9e01a9b6a2d27013726a07e7bcfcfd40/tumblr_p9yt8fsbd41wd0zf4o3_400.gif",
        "https://66.media.tumblr.com/9f0a8db1d9869651c1088ad696e286db/tumblr_p9yt8fsbd41wd0zf4o4_400.gif",
        "https://66.media.tumblr.com/bbb825e0fa2e246e5c08ba13ab253512/tumblr_p9yt8fsbd41wd0zf4o1_400.gif",
        "https://66.media.tumblr.com/fdbfaedac1eaa2b6195884d570cf03af/tumblr_p9yt8fsbd41wd0zf4o5_400.gif",
        "https://78.media.tumblr.com/3eb85f4176d7dbbaf99e6f2b7bd99e35/tumblr_p36oj12M7Z1uqi5u1o1_400.gif",
        "https://78.media.tumblr.com/b60c524ee0f19f9231677b575040e232/tumblr_og4ljpvuMl1u4ypbyo3_500.gif",
        "https://media.giphy.com/media/xUOxffzaOMBG2r22Yg/giphy.gif",
        "https://media1.tenor.com/images/06a5bd44234bfd672ae039f0c412d7a7/tenor.gif?itemid=10995354",
        "What up, ding-dongs? Yeah, so basically, um, the Fake Eleanor's a dirt bag, and these jabronis are gonna try and claim she's less of a dirt bag now, but she just stole your train, and she still sucks bad. And she belongs with us. Oh, also, check this out. [Farting] Nailed it.",
        "What's up, fork nuts?"
    }

    n := rand.Int() % len(quotes)

    return quotes[n]
}


func goodJanetQuote () {

    quotes := []string{
        "https://giphy.com/gifs/thegoodplace-episode-7-nbc-3oxHQwW2OulGir0Vry",
        "https://giphy.com/gifs/thegoodplace-season-1-episode-8-3oxHQyPgQ46eSib8ys",
        "https://giphy.com/gifs/thegoodplace-season-1-xUOxeZdzhJ5WuRqhRC",
        "https://giphy.com/gifs/thegoodplace-season-2-nbc-3ohs7Yw7tA7JwHppF6",
        "https://giphy.com/gifs/thegoodplace-season-2-nbc-xUOxeT5ZpZVStUPxC0",
        "https://img.buzzfeed.com/buzzfeed-static/static/2017-11/24/6/asset/buzzfeed-prod-fastlane-01/anigif_sub-buzz-4599-1511523128-1.gif",
        "https://media.giphy.com/media/3ohs7VBWnqm88MLmla/200w.gif",
        "https://media.giphy.com/media/xUOxeRRkTYdQJfyy2Y/200w.gif",
        "https://media.giphy.com/media/xUOxfeTChAZGoqRZ8A/200w.gif",
        "In case you were wondering, I am, by definition, the best version of myself.",
        "It turns out the best Janet was the Janet that was inside Janet all along.",
        "Not a girl!",
    }

    n := rand.Int() % len(quotes)

    return quotes[n]
}


func appendQuoteToMessage() bool {
    return rand.Float32() < 0.5
}