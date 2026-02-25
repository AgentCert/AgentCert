db.getSiblingDB("auth").users.updateOne(
  {username: "admin"}, 
  {$set: {password: "$2a$08$Qs37QPtj3qoYKqHVj7XwmO4NpPuBh6Zpe8YP0umTO0g3dXxIlaDPC"}}
)
